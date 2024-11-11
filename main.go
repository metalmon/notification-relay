package main

import (
	"context"
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
	fbApp              *firebase.App
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
	ctx := context.Background()
	opt := option.WithCredentialsFile(serviceAccountPath)
	var err error
	fbApp, err = firebase.NewApp(ctx, nil, opt)
	if err != nil {
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
	if trustedProxies == "*" {
		return router.SetTrustedProxies(nil) // Trust all proxies
	}

	if trustedProxies == "none" {
		return router.SetTrustedProxies([]string{}) // Trust no proxies
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
