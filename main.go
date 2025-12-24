package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
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

var initFirebase = func() error {
	if serviceAccountPath == "" {
		return fmt.Errorf("failed to initialize Firebase: no service account file found")
	}

	content, err := readAndValidateServiceAccount()
	if err != nil {
		return err
	}

	return initializeFirebaseApp(content)
}

func init() {
	// gin.SetMode(gin.ReleaseMode)
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

func validateServiceAccount(jsonContent map[string]interface{}) error {
	requiredFields := []string{
		"type",
		"project_id",
		"private_key_id",
		"private_key",
		"client_email",
		"client_id",
		"auth_uri",
		"token_uri",
		"auth_provider_x509_cert_url",
		"client_x509_cert_url",
	}

	for _, field := range requiredFields {
		if _, ok := jsonContent[field].(string); !ok {
			return fmt.Errorf("missing or invalid required field: %s", field)
		}
	}

	// Verify it's a service account
	if accountType, _ := jsonContent["type"].(string); accountType != "service_account" {
		return fmt.Errorf("invalid account type: %s", accountType)
	}

	return nil
}

func readAndValidateServiceAccount() ([]byte, error) {
	if _, err := os.Stat(serviceAccountPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to initialize Firebase: service account file not found")
		}
		return nil, fmt.Errorf("failed to initialize Firebase: error accessing service account file: %v", err)
	}

	// #nosec G304 -- serviceAccountPath is validated before use
	content, err := os.ReadFile(serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase: could not read service account file: %v", err)
	}

	return content, nil
}

func initializeFirebaseApp(content []byte) error {
	var jsonContent map[string]interface{}
	if err := json.Unmarshal(content, &jsonContent); err != nil {
		return fmt.Errorf("failed to initialize Firebase: invalid service account JSON: %v", err)
	}

	if err := validateServiceAccount(jsonContent); err != nil {
		return fmt.Errorf("failed to initialize Firebase: %v", err)
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

func getAllowedOrigins() []string {
	// Check environment variable first
	envOrigins := os.Getenv("ALLOWED_ORIGINS")
	log.Printf("[CORS] ALLOWED_ORIGINS env var: '%s'", envOrigins)

	if envOrigins != "" {
		// Special case for "*"
		if envOrigins == "*" {
			log.Printf("[CORS] Using wildcard (*) for allowed origins")
			return []string{"*"}
		}
		// Split comma-separated list
		origins := strings.Split(envOrigins, ",")
		log.Printf("[CORS] Using origins from env: %v", origins)
		return origins
	}

	// Fallback to config file
	if len(config.AllowedOrigins) > 0 {
		log.Printf("[CORS] Using origins from config: %v", config.AllowedOrigins)
		return config.AllowedOrigins
	}

	// Default to restrictive setting
	log.Printf("[CORS] No origins configured, using empty list")
	return []string{}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := getAllowedOrigins()

		log.Printf("[CORS] Request from origin: %s", origin)
		log.Printf("[CORS] Allowed origins: %v", allowedOrigins)
		log.Printf("[CORS] Request method: %s", c.Request.Method)
		log.Printf("[CORS] Request path: %s", c.Request.URL.Path)

		if !handleCORSOrigin(c, origin, allowedOrigins) {
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		// Log response headers
		log.Printf("[CORS] Response headers: %+v", c.Writer.Header())

		if c.Request.Method == "OPTIONS" {
			log.Printf("[CORS] Handling OPTIONS preflight request")
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

func handleCORSOrigin(c *gin.Context, origin string, allowedOrigins []string) bool {
	if origin == "" {
		log.Printf("[CORS] No origin header, using wildcard")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		return true
	}

	// Special case: allow all origins
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		log.Printf("[CORS] Wildcard origin configured, allowing: %s", origin)
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		return true
	}

	// Check if origin is in allowed list
	for _, allowed := range allowedOrigins {
		if allowed == origin {
			log.Printf("[CORS] Origin %s matched allowed origin %s", origin, allowed)
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			return true
		}
	}

	log.Printf("[CORS] Rejected request from unauthorized origin: %s. Allowed origins: %v",
		origin, allowedOrigins)
	c.AbortWithStatus(http.StatusForbidden)
	return false
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

	// Add CORS middleware with logging and origin validation
	router.Use(corsMiddleware())

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

	// Add this before setting up routes
	router.Use(func(c *gin.Context) {
		// Normalize double slashes in URL path
		path := strings.ReplaceAll(c.Request.URL.Path, "//", "/")
		if path != c.Request.URL.Path {
			c.Request.URL.Path = path
		}
		c.Next()
	})

	// API routes - make sure the path starts with a single slash
	router.GET("/api/method/notification_relay.api.get_config", getConfig)
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

	log.Printf("Starting server on port %s", port)
	if err := router.Run("0.0.0.0:" + port); err != nil {
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
