package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

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
	DecorationJSON      = "decoration.json"
	TopicDecorationJSON = "topic-decoration.json"
	// IconsJSON maps projects to their icon paths
	IconsJSON = "icons.json"
	// DefaultTrustedProxies defines default CIDR ranges for trusted proxies
	DefaultTrustedProxies = "127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
)

var (
	messagingClient    FirebaseMessagingClient
	config             Config
	userDeviceMap      = make(map[string]map[string][]string)
	decorations        = make(map[string]map[string]Decoration)
	topicDecorations   = make(map[string]TopicDecoration)
	icons              = make(map[string]string)
	serviceAccountPath string
	configPath         string
	// Whitelist of allowed configuration files
	allowedFiles = map[string]bool{
		ConfigJSON:          true,
		CredentialsJSON:     true,
		UserDeviceMapJSON:   true,
		DecorationJSON:      true,
		IconsJSON:           true,
		TopicDecorationJSON: true,
		"test.json":         true,
	}
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

func initFirebase() error {
	// Check if service account path exists in environment
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
	}

	// Check if we found a service account file
	if serviceAccountPath == "" {
		return fmt.Errorf("failed to initialize Firebase: no service account file found")
	}

	// Check if service account file exists
	if _, err := os.Stat(serviceAccountPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("failed to initialize Firebase: service account file not found")
		}
		return fmt.Errorf("failed to initialize Firebase: error accessing service account file: %v", err)
	}

	// Read service account file
	content, err := os.ReadFile(serviceAccountPath)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase: could not read service account file: %v", err)
	}

	// Verify it's valid JSON
	var jsonContent map[string]interface{}
	if err := json.Unmarshal(content, &jsonContent); err != nil {
		return fmt.Errorf("failed to initialize Firebase: invalid service account JSON: %v", err)
	}

	// Verify required fields
	requiredFields := []string{"type", "project_id", "private_key", "client_email"}
	for _, field := range requiredFields {
		if _, ok := jsonContent[field]; !ok {
			return fmt.Errorf("failed to initialize Firebase: missing required field in service account: %s", field)
		}
	}

	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceAccountPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("failed to initialize Firebase: %v", err)
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize messaging client: %v", err)
	}

	messagingClient = client
	return nil
}

// Config represents the application configuration structure
type Config struct {
	VapidPublicKey string                 `json:"vapid_public_key"`
	FirebaseConfig map[string]interface{} `json:"firebase_config"`
	TrustedProxies string                 `json:"trusted_proxies"`
}

func main() {
	// Load configuration
	if err := loadJSON(ConfigJSON, &config); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize Firebase
	if err := initFirebase(); err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}

	// Initialize credentials
	initCredentials()

	// Load other data files
	loadDataFiles()

	// Setup router
	router := gin.Default()

	// Configure trusted proxies
	trustedProxies := os.Getenv("TRUSTED_PROXIES")
	if trustedProxies == "" {
		trustedProxies = config.TrustedProxies // From config file
		if trustedProxies == "" {
			trustedProxies = DefaultTrustedProxies
		}
	}

	if err := setTrustedProxies(router, trustedProxies); err != nil {
		log.Printf("Warning: Failed to set trusted proxies: %v", err)
	}

	// API routes
	router.POST("/api/method/notification_relay.api.auth.get_credential", getCredential)
	router.POST("/api/method/notification_relay.api.get_config", getConfig)

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

func setTrustedProxies(router *gin.Engine, trustedProxies string) error {
	trustedProxies = strings.TrimSpace(trustedProxies)

	if trustedProxies == "" || trustedProxies == "none" {
		return router.SetTrustedProxies([]string{}) // Trust no proxies
	}

	if trustedProxies == "*" {
		return router.SetTrustedProxies(nil) // Trust all proxies
	}

	// Split the comma-separated list
	proxyList := strings.Split(trustedProxies, ",")

	// Validate each CIDR
	for _, proxy := range proxyList {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		_, _, err := net.ParseCIDR(proxy)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q: %v", proxy, err)
		}
	}

	return router.SetTrustedProxies(proxyList)
}
