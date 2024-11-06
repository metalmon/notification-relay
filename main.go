package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

type Config struct {
	VapidPublicKey   string                 `json:"vapid_public_key"`
	FirebaseConfig   map[string]interface{} `json:"firebase_config"`
	APIKey          string
	APISecret       string
}

type UserDeviceMap map[string]map[string][]string

type DecorationRule struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

type ProjectDecoration map[string]DecorationRule
type Decorations map[string]ProjectDecoration

type Response struct {
	Message *SuccessResponse `json:"message,omitempty"`
	Error   *ErrorResponse   `json:"exc,omitempty"`
}

type SuccessResponse struct {
	Success int    `json:"success"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

var (
	userDeviceMap UserDeviceMap
	decorations   Decorations
	icons        map[string]string
	fbApp        *firebase.App
	config       Config
)

func init() {
	// Load configurations
	loadConfig()
	
	// Initialize Firebase
	opt := option.WithCredentialsJSON([]byte(config.FirebaseConfig))
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}
	fbApp = app

	// Load data files
	loadDataFiles()
}

func main() {
	r := gin.Default()
	
	// Add authentication middleware
	auth := r.Group("/", basicAuth())

	// Routes
	auth.GET("/api/method/notification_relay.api.get_config", getConfig)
	auth.POST("/api/method/notification_relay.api.topic.subscribe", subscribeToTopic)
	auth.POST("/api/method/notification_relay.api.topic.unsubscribe", unsubscribeFromTopic)
	auth.POST("/api/method/notification_relay.api.token.add", addToken)
	auth.POST("/api/method/notification_relay.api.token.remove", removeToken)
	auth.POST("/api/method/notification_relay.api.send_notification.user", sendNotificationToUser)
	auth.POST("/api/method/notification_relay.api.send_notification.topic", sendNotificationToTopic)
	auth.POST("/api/method/notification_relay.api.auth.get_credential", getCredential)

	r.Run(":5000")
} 