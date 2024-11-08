package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

const (
	ConfigJSON        = "config.json"
	CredentialsJSON   = "credentials.json"
	UserDeviceMapJSON = "user-device-map.json"
	DecorationJSON    = "decoration.json"
	IconsJSON         = "icons.json"
)

type Config struct {
	VapidPublicKey string                 `json:"vapid_public_key"`
	FirebaseConfig map[string]interface{} `json:"firebase_config"`
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

type Icons map[string]string

var (
	userDeviceMap UserDeviceMap
	decorations   Decorations
	icons         Icons
	fbApp         *firebase.App
	config        Config

	// Version information - these will be set during build by ldflags
	gitTag    = "unknown"
	gitCommit = "unknown"
	gitBranch = "unknown"
)

func apiBasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, apiSecret, hasAuth := c.Request.BasicAuth()

		if !hasAuth {
			c.Header("WWW-Authenticate", "Basic realm=Authorization Required")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if storedSecret, exists := credentials[apiKey]; !exists || storedSecret != apiSecret {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

// InitializeApp initializes the application with the given config path
func InitializeApp() {
	// Log version information at startup
	log.Printf("Version: %s, Commit: %s, Branch: %s", gitTag, gitCommit, gitBranch)

	// Load configurations
	var err error
	configBytes, err := os.ReadFile(getConfigPath(ConfigJSON))
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	if err := json.Unmarshal(configBytes, &config); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	// Initialize Firebase
	firebaseConfigJSON, err := json.Marshal(config.FirebaseConfig)
	if err != nil {
		log.Fatalf("Failed to marshal Firebase config: %v", err)
	}

	opt := option.WithCredentialsJSON(firebaseConfigJSON)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}
	fbApp = app

	// Load all data files
	loadDataFiles()

	// Initialize credentials last
	initCredentials()
}

func main() {
	// Log version information
	log.Printf("Version: %s, Commit: %s, Branch: %s", gitTag, gitCommit, gitBranch)

	r := gin.Default()

	// Add authentication middleware
	auth := r.Group("/", apiBasicAuth())

	// Routes
	auth.POST("/api/method/notification_relay.api.auth.get_credential", getCredential)
	auth.POST("/api/method/notification_relay.api.topic.subscribe", subscribeToTopic)
	auth.POST("/api/method/notification_relay.api.topic.unsubscribe", unsubscribeFromTopic)
	auth.POST("/api/method/notification_relay.api.token.add", addToken)
	auth.POST("/api/method/notification_relay.api.token.remove", removeToken)
	auth.POST("/api/method/notification_relay.api.send_notification.user", sendNotificationToUser)
	auth.POST("/api/method/notification_relay.api.send_notification.topic", sendNotificationToTopic)

	// Get port from environment variable or use default
	port := os.Getenv("LISTEN_PORT")
	if port == "" {
		port = "5000"
	}

	// Run the server and handle potential errors
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
