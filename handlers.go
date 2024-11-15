package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
)

// CredentialRequest represents a request for API credentials with validation requirements
type CredentialRequest struct {
	Endpoint     string `json:"endpoint"`      // Required: The endpoint URL
	Protocol     string `json:"protocol"`      // The protocol (http/https)
	Port         string `json:"port"`          // Optional: The port number
	Token        string `json:"token"`         // Required: Authentication token
	WebhookRoute string `json:"webhook_route"` // The webhook route path
}

// CredentialResponse represents an API credentials response
type CredentialResponse struct {
	Success     bool               `json:"success"`
	Message     string             `json:"message,omitempty"`
	Credentials *CredentialDetails `json:"credentials,omitempty"`
	Exc         string             `json:"exc,omitempty"`
}

// CredentialDetails contains generated API credentials
type CredentialDetails struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// Credentials represents a map of API credentials
type Credentials map[string]string

// Response represents the standard API response structure with optional fields
type Response struct {
	Message interface{} `json:"message,omitempty"` // Success message or status information
	Data    interface{} `json:"data,omitempty"`    // Response payload data
	Exc     string      `json:"exc,omitempty"`     // Error message for critical failures
}

// Decoration represents a notification title decoration rule for user notifications
type Decoration struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// TopicDecoration represents a notification title decoration rule for topic notifications
type TopicDecoration struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// ConfigResponse represents the response structure for the getConfig endpoint
type ConfigResponse struct {
	VapidPublicKey string                 `json:"vapid_public_key"`
	Config         map[string]interface{} `json:"config"`
	Exc            string                 `json:"exc,omitempty"`
}

// NotificationPayload represents the structure of the notification payload
type NotificationPayload struct {
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Data        map[string]string `json:"data,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	ClickAction string            `json:"click_action,omitempty"`
}

// getConfig returns the VAPID public key and Firebase configuration
func getConfig(c *gin.Context) {
	// Project name is accepted but not used
	projectName := c.Query("project_name")
	log.Printf("Get config request for project: %s", projectName)

	// Log the current config state
	log.Printf("Current config state - VapidPublicKey: %s", config.VapidPublicKey)
	log.Printf("Current config state - FirebaseConfig: %+v", config.FirebaseConfig)

	// Check if required configuration is available
	if config.VapidPublicKey == "" {
		log.Printf("Error: VAPID public key is empty")
		c.JSON(http.StatusInternalServerError, gin.H{
			"exc": "VAPID public key not configured",
		})
		return
	}

	if config.FirebaseConfig == nil {
		log.Printf("Error: Firebase config is nil")
		c.JSON(http.StatusInternalServerError, gin.H{
			"exc": "Firebase configuration not initialized",
		})
		return
	}

	response := gin.H{
		"vapid_public_key": config.VapidPublicKey,
		"config":           config.FirebaseConfig,
	}

	// Log the response we're sending
	log.Printf("Sending response: %+v", response)

	c.JSON(http.StatusOK, response)
}

// getCredential handles API credential requests by validating the request,
// verifying the provided token by making a request to the site's webhook,
// and returning API credentials if verification is successful.
// It expects a CredentialRequest with endpoint, protocol, port, token and webhook route.
// Returns a CredentialResponse with success status and either credentials or error message.
func getCredential(c *gin.Context) {
	var req CredentialRequest

	// Try to get parameters from query string first
	if c.Query("endpoint") != "" {
		req = CredentialRequest{
			Endpoint:     c.Query("endpoint"),
			Protocol:     c.Query("protocol"),
			Port:         c.Query("port"),
			Token:        c.Query("token"),
			WebhookRoute: c.Query("webhook_route"),
		}
	} else {
		// Fall back to JSON body if query parameters aren't present
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusOK, CredentialResponse{
				Success: false,
				Message: "Invalid request format",
			})
			return
		}
	}

	// Log the request details
	log.Printf("Credential request - Endpoint: %s, Protocol: %s, Port: %s, Token: %s, WebhookRoute: %s",
		req.Endpoint, req.Protocol, req.Port, req.Token, req.WebhookRoute)

	// Validate the request
	if req.Endpoint == "" || req.Token == "" {
		c.JSON(http.StatusOK, CredentialResponse{
			Success: false,
			Message: "Missing required fields",
		})
		return
	}

	// Create HTTP client with options based on the endpoint
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// For localhost, if protocol is HTTPS:
	// Option 1: Switch to HTTP
	webhookProtocol := req.Protocol
	if req.Endpoint == "localhost" && req.Protocol == "https" {
		log.Printf("Switching to HTTP for localhost")
		webhookProtocol = "http"
	}

	// Option 2: Or skip TLS verification for localhost
	/*
		if req.Endpoint == "localhost" {
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	*/

	// Verify token by making request to the site's webhook
	webhookURL := fmt.Sprintf("%s://%s%s%s",
		webhookProtocol, // Use modified protocol
		req.Endpoint,
		func() string {
			if req.Port != "" {
				return ":" + req.Port
			}
			return ""
		}(),
		req.WebhookRoute,
	)

	log.Printf("Making webhook request to: %s", webhookURL)

	resp, err := client.Get(webhookURL)
	if err != nil {
		log.Printf("Webhook request failed: %v", err)
		c.JSON(http.StatusOK, CredentialResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to verify token: %v", err),
		})
		return
	}

	if resp != nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
		}()
	}

	log.Printf("Webhook response status: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, CredentialResponse{
			Success: false,
			Message: "Token verification failed",
		})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading webhook response: %v", err)
		c.JSON(http.StatusOK, CredentialResponse{
			Success: false,
			Message: "Invalid token",
		})
		return
	}

	log.Printf("Webhook response body: %s", string(body))
	log.Printf("Expected token: %s", req.Token)

	if string(body) != req.Token {
		c.JSON(http.StatusOK, CredentialResponse{
			Success: false,
			Message: "Invalid token",
		})
		return
	}

	// Generate new API credentials
	apiKey := generateSecureToken(32)
	apiSecret := generateSecureToken(48)

	// Store credentials in the map and save to file
	credentials[apiKey] = apiSecret
	err = saveJSON(CredentialsJSON, credentials)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"exc": gin.H{
				"status_code": 500,
				"message":     fmt.Sprintf("Failed to save credentials: %v", err),
			},
		})
		return
	}

	log.Printf("Generated credentials - APIKey: %s", apiKey)

	// Return response in Python server format
	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"success": true,
			"credentials": gin.H{
				"api_key":    apiKey,
				"api_secret": apiSecret,
			},
		},
	})
}

// generateSecureToken generates a cryptographically secure random token of the specified length
// using characters from the charset (a-z, A-Z, 0-9). Returns the generated token as a string.
func generateSecureToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			log.Printf("Error generating random number: %v", err)
			continue
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// apiBasicAuth returns a middleware handler that performs Basic Auth validation
// using API credentials stored in the credentials map.
func apiBasicAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, apiSecret, hasAuth := c.Request.BasicAuth()
		if !hasAuth {
			c.Header("WWW-Authenticate", "Basic realm=Authorization Required")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if storedSecret, exists := credentials[apiKey]; !exists || storedSecret != apiSecret {
			c.Header("WWW-Authenticate", "Basic realm=Authorization Required")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

// subscribeToTopic subscribes a user's devices to a Firebase topic.
// Takes project name, site name, user ID and topic name from query parameters.
// Retrieves user's FCM tokens and subscribes them to the specified topic.
// Returns success response if subscription succeeds, error response otherwise.
func subscribeToTopic(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	topicName := c.Query("topic_name")

	// Check if topic name is empty first
	if topicName == "" {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": "topic_name is required",
			},
		})
		return
	}

	// Get user tokens first using getUserTokens
	tokens, err := getUserTokens(key, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 404,
				"message":     err.Error(),
			},
		})
		return
	}

	// Subscribe tokens to topic
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := messagingClient.SubscribeToTopic(ctx, tokens, topicName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 400,
				"message":     fmt.Sprintf("Failed to subscribe to topic: %v", err),
			},
		})
		return
	}

	// Log subscription result
	log.Printf("Topic subscription result - Success: %d, Failures: %d", response.SuccessCount, response.FailureCount)

	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"success": 200,
			"message": fmt.Sprintf("User subscribed to topic %s. Success: %d, Failures: %d",
				topicName, response.SuccessCount, response.FailureCount),
		},
	})
}

// unsubscribeFromTopic unsubscribes a user's devices from a Firebase topic.
// Takes project name, site name, user ID and topic name from query parameters.
// Retrieves user's FCM tokens and unsubscribes them from the specified topic.
// Returns success response if unsubscription succeeds, error response otherwise.
func unsubscribeFromTopic(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	topicName := c.Query("topic_name")

	// Check if topic name is empty
	if topicName == "" {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": "topic_name is required",
			},
		})
		return
	}

	if tokens, exists := userDeviceMap[key][userID]; exists && len(tokens) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := messagingClient.UnsubscribeFromTopic(ctx, tokens, topicName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"exc": gin.H{
					"status_code": 400,
					"message":     fmt.Sprintf("Failed to unsubscribe from topic: %v", err),
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": gin.H{
				"success": 200,
				"message": fmt.Sprintf("User %s unsubscribed from %s topic", userID, topicName),
			},
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"exc": gin.H{
			"status_code": 404,
			"message":     fmt.Sprintf("%s not subscribed to push notifications", userID),
		},
	})
}

// addToken adds a user's FCM token to the user's device map.
// Takes project name, site name, user ID and FCM token from query parameters.
// Checks for duplicate tokens and saves the token to the user's device map.
// Returns success response if token is added, error response otherwise.
func addToken(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	fcmToken := c.Query("fcm_token")

	// Check if token is empty
	if fcmToken == "" {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": "FCM token is required",
			},
		})
		return
	}

	if userDeviceMap[key] == nil {
		userDeviceMap[key] = make(map[string][]string)
	}

	if tokens, exists := userDeviceMap[key][userID]; exists {
		// Check for duplicate token
		for _, token := range tokens {
			if token == fcmToken {
				c.JSON(http.StatusOK, gin.H{
					"message": gin.H{
						"success": 200,
						"message": "User Token duplicate found",
					},
				})
				return
			}
		}
		userDeviceMap[key][userID] = append(tokens, fcmToken)
	} else {
		userDeviceMap[key][userID] = []string{fcmToken}
	}

	err := saveJSON(UserDeviceMapJSON, userDeviceMap)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"exc": gin.H{
				"status_code": 500,
				"message":     fmt.Sprintf("Failed to save user device map: %v", err),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"success": 200,
			"message": "User Token added",
		},
	})
}

// removeToken removes a user's FCM token from the user's device map.
// Takes project name, site name, user ID and FCM token from query parameters.
// Removes the token from the user's device map and saves the updated map to the file.
// Returns success response if token is removed, error response otherwise.
func removeToken(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	fcmToken := c.Query("fcm_token")

	if tokens, exists := userDeviceMap[key][userID]; exists {
		for i, token := range tokens {
			if token == fcmToken {
				userDeviceMap[key][userID] = append(tokens[:i], tokens[i+1:]...)
				err := saveJSON(UserDeviceMapJSON, userDeviceMap)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"exc": gin.H{
							"status_code": 500,
							"message":     "Failed to save user device map",
						},
					})
					return
				}
				c.JSON(http.StatusOK, gin.H{
					"message": gin.H{
						"success": 200,
						"message": "User Token removed",
					},
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"success": 200,
			"message": "User Token not found, removed",
		},
	})
}

// getUserTokens retrieves the user's tokens
func getUserTokens(key, userID string) ([]string, error) {
	tokens, exists := userDeviceMap[key][userID]
	if !exists || len(tokens) == 0 {
		return nil, fmt.Errorf("user %s not subscribed to push notifications", userID)
	}
	return tokens, nil
}

// validateNotificationParams checks the required parameters for a notification
func validateNotificationParams(title, body string) error {
	if title == "" {
		return fmt.Errorf("title is required")
	}
	if body == "" {
		return fmt.Errorf("body is required")
	}
	return nil
}

// applyDecorations applies decorations to the notification title based on project settings
func applyDecorations(key, title string) string {
	if projectDecorations, exists := decorations[key]; exists {
		for _, decoration := range projectDecorations {
			matched, err := regexp.MatchString(decoration.Pattern, title)
			if err != nil {
				log.Printf("Error matching pattern: %v", err)
				continue
			}
			if matched {
				return strings.Replace(decoration.Template, "{title}", title, 1)
			}
		}
	}
	return title
}

// addIconToConfig adds the project icon to the webpush configuration
func addIconToConfig(key string, webpushConfig *messaging.WebpushConfig) {
	if iconPath, exists := icons[key]; exists {
		if webpushConfig.Data == nil {
			webpushConfig.Data = make(map[string]string)
		}
		webpushConfig.Data["icon"] = iconPath
	}
}

// prepareWebPushConfig creates a web push notification configuration for a user.
// Applies decorations to the title, adds icon, and processes additional data.
// Returns the prepared configuration and an error if data processing fails.
func prepareWebPushConfig(key, title, body, data string) (*messaging.WebpushConfig, error) {
	decoratedTitle := applyDecorations(key, title)

	webpushConfig := &messaging.WebpushConfig{
		Notification: &messaging.WebpushNotification{
			Title: decoratedTitle,
			Body:  body,
		},
	}

	if data != "" {
		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
			return nil, fmt.Errorf("invalid data format: %v", err)
		}

		if clickAction, ok := dataMap["click_action"].(string); ok {
			webpushConfig.FCMOptions = &messaging.WebpushFCMOptions{
				Link: clickAction,
			}
		}
	}

	addIconToConfig(key, webpushConfig)
	return webpushConfig, nil
}

// sendNotificationToUser sends a web push notification to all user's devices.
// Takes notification parameters from request query parameters.
// Returns a JSON response with the sending result.
func sendNotificationToUser(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

	// Get user's tokens first
	tokens, err := getUserTokens(key, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 404,
				"message":     err.Error(),
			},
		})
		return
	}

	// Parse the data JSON for notification settings
	var dataMap map[string]interface{}
	if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 400,
				"message":     fmt.Sprintf("Invalid data format: %v", err),
			},
		})
		return
	}

	// Check if the notification is for the sender
	if fromUser, ok := dataMap["from_user"].(string); ok && fromUser == userID {
		// Skip sending notification to self
		c.JSON(http.StatusOK, gin.H{
			"message": gin.H{
				"success": 200,
				"message": "Skipped sending notification to self",
			},
		})
		return
	}

	// Convert data fields to string values for FCM
	dataMapStr := convertToStringMap(dataMap)

	// Get notification icon and click_action from original dataMap
	notificationIcon := ""
	if icon, ok := dataMap["notification_icon"].(string); ok {
		notificationIcon = icon
	}

	// Prepare web push config
	webpushConfig := &messaging.WebpushConfig{
		Notification: &messaging.WebpushNotification{
			Title: title,
			Body:  body,
			Icon:  notificationIcon,
		},
	}

	// Add click_action if present in original dataMap
	if clickAction, ok := dataMap["click_action"].(string); ok {
		webpushConfig.FCMOptions = &messaging.WebpushFCMOptions{
			Link: clickAction,
		}
	}

	// Add icon if configured
	addIconToConfig(key, webpushConfig)

	// Create a map for notification data
	notificationData := map[string]string{
		"title": title,
		"body":  body,
	}

	// Add all fields from dataMap to notificationData
	for k, v := range dataMapStr {
		notificationData[k] = v
	}

	// Send notification to all user tokens
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	validTokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		message := &messaging.Message{
			Token:   token,
			Webpush: webpushConfig,
			Data:    notificationData, // Send combined data
		}

		// Log the outgoing message
		log.Printf("Sending FCM message to token %s:", token)
		if msgBytes, err := json.MarshalIndent(message, "", "  "); err == nil {
			log.Printf("Message payload:\n%s", string(msgBytes))
		}

		// Send the message
		response, err := messagingClient.Send(ctx, message)
		if err != nil {
			log.Printf("Failed to send notification to token %s: %v", token, err)
			continue
		}

		log.Printf("FCM Response for token %s: %s", token, response)
		validTokens = append(validTokens, token)
	}

	// Return response
	if len(validTokens) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": gin.H{
				"success": 200,
				"message": fmt.Sprintf("%d Notification(s) sent to %s user", len(validTokens), userID),
			},
		})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"exc": gin.H{
			"status_code": 404,
			"message":     fmt.Sprintf("%s not subscribed to push notifications", userID),
		},
	})
}

// applyTopicDecorations applies decorations to the notification title based on topic
func applyTopicDecorations(topic, title string) string {
	if decoration, exists := topicDecorations[topic]; exists {
		matched, err := regexp.MatchString(decoration.Pattern, title)
		if err != nil {
			log.Printf("Error matching pattern: %v", err)
			return title
		}
		if matched {
			return strings.Replace(decoration.Template, "{title}", title, 1)
		}
	}
	return title
}

// prepareTopicWebPushConfig creates a web push notification configuration for a topic
func prepareTopicWebPushConfig(topic, title, body, data string) (*messaging.WebpushConfig, error) {
	// Apply decorations to the title based on topic
	decoratedTitle := applyTopicDecorations(topic, title)

	webpushConfig := &messaging.WebpushConfig{
		Notification: &messaging.WebpushNotification{
			Title: decoratedTitle,
			Body:  body,
		},
	}

	if data != "" {
		var dataMap map[string]interface{}
		if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
			return nil, fmt.Errorf("invalid data format: %v", err)
		}

		if clickAction, ok := dataMap["click_action"].(string); ok {
			webpushConfig.FCMOptions = &messaging.WebpushFCMOptions{
				Link: clickAction,
			}
		}

		if icon, ok := dataMap["icon"].(string); ok {
			if webpushConfig.Data == nil {
				webpushConfig.Data = make(map[string]string)
			}
			webpushConfig.Data["icon"] = icon
		}
	}

	return webpushConfig, nil
}

// sendNotificationToTopic sends a web push notification to a Firebase topic.
// Takes topic name, title, body and additional data from query parameters.
// Returns a JSON response with the sending result.
func sendNotificationToTopic(c *gin.Context) {
	topic := c.Query("topic_name")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

	// Check if topic name is empty
	if topic == "" {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": "topic_name is required",
			},
		})
		return
	}

	// Check required parameters
	if err := validateNotificationParams(title, body); err != nil {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		})
		return
	}

	// Parse the data JSON for notification settings
	var dataMap map[string]interface{}
	if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 400,
				"message":     fmt.Sprintf("Invalid data format: %v", err),
			},
		})
		return
	}

	// Check if the notification is from the current user
	if fromUser, ok := dataMap["from_user"].(string); ok {
		// Get current user's tokens
		projectName := c.Query("project_name")
		siteName := c.Query("site_name")
		key := projectName + "_" + siteName

		// Get user's tokens
		if tokens, exists := userDeviceMap[key][fromUser]; exists && len(tokens) > 0 {
			// Unsubscribe sender's tokens temporarily
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err := messagingClient.UnsubscribeFromTopic(ctx, tokens, topic)
			if err != nil {
				log.Printf("Failed to unsubscribe sender from topic: %v", err)
			}

			// Send notification
			err = sendTopicNotification(c, topic, title, body, dataMap)

			// Resubscribe sender's tokens
			_, resubErr := messagingClient.SubscribeToTopic(ctx, tokens, topic)
			if resubErr != nil {
				log.Printf("Failed to resubscribe sender to topic: %v", resubErr)
			}

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"exc": gin.H{
						"status_code": 500,
						"message":     fmt.Sprintf("Failed to send notification: %v", err),
					},
				})
				return
			}
		} else {
			// If sender's tokens not found, just send the notification
			err := sendTopicNotification(c, topic, title, body, dataMap)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"exc": gin.H{
						"status_code": 500,
						"message":     fmt.Sprintf("Failed to send notification: %v", err),
					},
				})
				return
			}
		}
	} else {
		// If no sender info, just send the notification
		err := sendTopicNotification(c, topic, title, body, dataMap)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"exc": gin.H{
					"status_code": 500,
					"message":     fmt.Sprintf("Failed to send notification: %v", err),
				},
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"success": 200,
			"message": fmt.Sprintf("Notification sent to %s topic", topic),
		},
	})
}

// Helper function to send topic notification
func sendTopicNotification(c *gin.Context, topic, title, body string, dataMap map[string]interface{}) error {
	// Convert data fields to string values for FCM
	dataMapStr := convertToStringMap(dataMap)

	// Get notification icon
	notificationIcon := ""
	if icon, ok := dataMap["notification_icon"].(string); ok {
		notificationIcon = icon
	}

	// Prepare web push config
	webpushConfig := &messaging.WebpushConfig{
		Notification: &messaging.WebpushNotification{
			Title: title,
			Body:  body,
			Icon:  notificationIcon,
			Tag:   fmt.Sprintf("notification-%d", time.Now().UnixNano()),
		},
	}

	// Add click_action if present
	if clickAction, ok := dataMap["click_action"].(string); ok {
		webpushConfig.FCMOptions = &messaging.WebpushFCMOptions{
			Link: clickAction,
		}
	}

	// Create notification data
	notificationData := map[string]string{
		"title": title,
		"body":  body,
	}
	for k, v := range dataMapStr {
		notificationData[k] = v
	}

	message := &messaging.Message{
		Topic:   topic,
		Webpush: webpushConfig,
		Data:    notificationData,
	}

	// Log the outgoing message
	if msgBytes, err := json.MarshalIndent(message, "", "  "); err == nil {
		log.Printf("Message payload:\n%s", string(msgBytes))
	}

	ctx := context.Background()
	response, err := messagingClient.Send(ctx, message)
	if err != nil {
		return err
	}

	log.Printf("FCM Response for topic %s: %s", topic, response)
	return nil
}

// Convert map[string]interface{} to map[string]string
func convertToStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = val
		default:
			// Convert other types to string using JSON marshaling
			if bytes, err := json.Marshal(val); err == nil {
				result[k] = string(bytes)
			}
		}
	}
	return result
}
