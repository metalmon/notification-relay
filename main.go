package main

import (
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

// Configuration file names
const (
	// ConfigJSON is the main configuration file
	ConfigJSON = "config.json"
	// CredentialsJSON stores API credentials
	CredentialsJSON = "credentials.json"
	// UserDeviceMapJSON maps users to their device tokens
	UserDeviceMapJSON = "user-device-map.json"
	// DecorationJSON contains notification decoration rules
	DecorationJSON = "decoration.json"
	// IconsJSON maps projects to their icon paths
	IconsJSON = "icons.json"
)

var (
	fbApp              *firebase.App
	config             Config
	userDeviceMap      = make(map[string]map[string][]string)
	decorations        = make(map[string][]Decoration)
	icons              = make(map[string]string)
	serviceAccountPath string
)

func init() {
	// Check for service account path in environment
	serviceAccountPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if serviceAccountPath == "" {
		// Default paths in order of preference
		paths := []string{
			"./service-account.json",
			"/etc/notification-relay/service-account.json",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				serviceAccountPath = path
				break
			}
		}
		if serviceAccountPath == "" {
			log.Fatal("No service account file found")
		}
	}
}

// Config represents the application configuration structure
type Config struct {
	VapidPublicKey string                 `json:"vapid_public_key"`
	FirebaseConfig map[string]interface{} `json:"firebase_config"`
}

func main() {
	// Load configuration
	if err := loadJSON(ConfigJSON, &config); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Firebase
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceAccountPath)
	var err error
	fbApp, err = firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}

	// Initialize Firebase Messaging
	if err := initMessaging(fbApp); err != nil {
		log.Fatalf("Failed to initialize Firebase Messaging: %v", err)
	}

	// Initialize credentials
	initCredentials()

	// Load other data files
	loadDataFiles()

	// Setup router
	router := gin.Default()

	// API routes
	router.POST("/api/method/notification_relay.api.auth.get_credential", getCredential)

	// Protected routes
	auth := router.Group("/", apiBasicAuth())
	auth.POST("/api/method/notification_relay.api.topic.subscribe", subscribeToTopic)
	auth.POST("/api/method/notification_relay.api.topic.unsubscribe", unsubscribeFromTopic)
	auth.POST("/api/method/notification_relay.api.token.add", addToken)
	auth.POST("/api/method/notification_relay.api.token.remove", removeToken)
	auth.POST("/api/method/notification_relay.api.send_notification.user", sendNotificationToUser)
	auth.POST("/api/method/notification_relay.api.send_notification.topic", sendNotificationToTopic)

	// Start server
	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		port = "5000"
	}
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
