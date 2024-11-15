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

func getConfig(c *gin.Context) {
	projectName := c.Query("project_name")
	log.Printf("Get config request for project: %s", projectName)

	if projectName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": http.StatusBadRequest,
				"message":     "Project name is required",
			},
		})
		return
	}

	// Get project-specific config
	projectConfig, exists := config.Projects[projectName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"exc": gin.H{
				"status_code": http.StatusNotFound,
				"message":     fmt.Sprintf("Configuration not found for project: %s", projectName),
			},
		})
		return
	}

	// Check if required configuration is available
	if projectConfig.VapidPublicKey == "" {
		log.Printf("Error: VAPID public key is empty for project %s", projectName)
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": http.StatusBadRequest,
				"message":     "VAPID public key not configured",
			},
		})
		return
	}

	if projectConfig.FirebaseConfig == nil {
		log.Printf("Error: Firebase config is nil for project %s", projectName)
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": http.StatusBadRequest,
				"message":     "Firebase configuration not initialized",
			},
		})
		return
	}

	// Return project-specific config
	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"vapid_public_key": projectConfig.VapidPublicKey,
			"config":           projectConfig.FirebaseConfig,
		},
	})
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
	key := formatProjectKey(projectName, siteName)
	userID := c.Query("user_id")
	topicName := c.Query("topic_name")

	// Validate project exists
	if err := validateProject(projectName); err != nil {
		sendErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	// Check if topic name is empty
	if topicName == "" {
		sendErrorResponse(c, http.StatusBadRequest, "topic_name is required")
		return
	}

	// Get user tokens
	tokens, err := getUserTokens(key, userID)
	if err != nil {
		sendErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	// Subscribe tokens to topic
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := messagingClient.SubscribeToTopic(ctx, tokens, topicName)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to subscribe to topic: %v", err))
		return
	}

	// Log subscription result
	log.Printf("Topic subscription result - Success: %d, Failures: %d", response.SuccessCount, response.FailureCount)

	sendSuccessResponse(c, fmt.Sprintf("User subscribed to topic %s. Success: %d, Failures: %d",
		topicName, response.SuccessCount, response.FailureCount))
}

// Add standardized error response helper
func sendErrorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"exc": gin.H{
			"status_code": statusCode,
			"message":     message,
		},
	})
}

// Add standardized success response helper
func sendSuccessResponse(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"message": gin.H{
			"success": 200,
			"message": message,
		},
	})
}

// Update unsubscribeFromTopic to use standard responses
func unsubscribeFromTopic(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := fmt.Sprintf("%s_%s", projectName, siteName)
	userID := c.Query("user_id")
	topicName := c.Query("topic_name")

	// Validate project exists
	if _, exists := config.Projects[projectName]; !exists {
		sendErrorResponse(c, http.StatusNotFound, fmt.Sprintf("Project %s not found", projectName))
		return
	}

	// Check if topic name is empty
	if topicName == "" {
		sendErrorResponse(c, http.StatusBadRequest, "topic_name is required")
		return
	}

	tokens, exists := userDeviceMap[key][userID]
	if !exists || len(tokens) == 0 {
		sendErrorResponse(c, http.StatusNotFound, fmt.Sprintf("%s not subscribed to push notifications", userID))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := messagingClient.UnsubscribeFromTopic(ctx, tokens, topicName)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to unsubscribe from topic: %v", err))
		return
	}

	sendSuccessResponse(c, fmt.Sprintf("User %s unsubscribed from %s topic", userID, topicName))
}

// addToken adds a user's FCM token to the user's device map.
// Takes project name, site name, user ID and FCM token from query parameters.
// Checks for duplicate tokens and saves the token to the user's device map.
// Returns success response if token is added, error response otherwise.
func addToken(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := formatProjectKey(projectName, siteName)
	userID := c.Query("user_id")
	fcmToken := c.Query("fcm_token")

	// Validate project exists in config
	if _, exists := config.Projects[projectName]; !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 404,
				"message":     fmt.Sprintf("Project %s not found", projectName),
			},
		})
		return
	}

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

	// Initialize project map if it doesn't exist
	if userDeviceMap[key] == nil {
		userDeviceMap[key] = make(map[string][]string)
	}

	// Add token to user's devices
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

	// Save updated map
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
	key := fmt.Sprintf("%s_%s", projectName, siteName)
	userID := c.Query("user_id")
	fcmToken := c.Query("fcm_token")

	// Validate project exists
	if _, exists := config.Projects[projectName]; !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 404,
				"message":     fmt.Sprintf("Project %s not found", projectName),
			},
		})
		return
	}

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
	// Split key to get project name
	parts := strings.Split(key, "_")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}
	projectName := parts[0]

	// Validate project exists
	if _, exists := config.Projects[projectName]; !exists {
		return nil, fmt.Errorf("project %s not found", projectName)
	}

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

// prepareWebPushConfig creates a web push notification configuration.
// Applies decorations to the title, adds icon, and processes additional data.
// If topic is provided, applies topic-specific decorations instead of project decorations.
func prepareWebPushConfig(key, title, body, data string, topic string) (*messaging.WebpushConfig, error) {
	// Apply decorations based on whether it's a topic notification or not
	decoratedTitle := title
	if topic != "" {
		decoratedTitle = applyTopicDecorations(topic, title)
	} else {
		decoratedTitle = applyDecorations(key, title)
	}

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

		// Handle icon from data for topic notifications
		if topic != "" {
			if icon, ok := dataMap["icon"].(string); ok {
				if webpushConfig.Data == nil {
					webpushConfig.Data = make(map[string]string)
				}
				webpushConfig.Data["icon"] = icon
			}
		}
	}

	// Only add project icon for non-topic notifications
	if topic == "" {
		addIconToConfig(key, webpushConfig)
	}

	return webpushConfig, nil
}

// Add logging helpers
func logNotificationSent(messageType, recipient string, message *messaging.Message) {
	if msgBytes, err := json.MarshalIndent(message, "", "  "); err == nil {
		log.Printf("Sending %s notification to %s:\n%s", messageType, recipient, string(msgBytes))
	}
}

func logNotificationResponse(messageType, recipient, response string) {
	log.Printf("FCM Response for %s %s: %s", messageType, recipient, response)
}

// Add helper for common notification data parsing
func parseNotificationData(data string) (map[string]interface{}, error) {
	var dataMap map[string]interface{}
	if data == "" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
		return nil, fmt.Errorf("invalid data format: %v", err)
	}
	return dataMap, nil
}

// Update send functions to use helper
func sendNotificationToUser(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := fmt.Sprintf("%s_%s", projectName, siteName)
	userID := c.Query("user_id")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

	// Validate notification parameters
	if err := validateNotificationParams(title, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"exc": gin.H{
				"status_code": 400,
				"message":     err.Error(),
			},
		})
		return
	}

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

	// Prepare web push config (pass empty topic string)
	webpushConfig, err := prepareWebPushConfig(key, title, body, data, "")
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to prepare notification: %v", err))
		return
	}

	// Parse the data for notification settings
	dataMap, err := parseNotificationData(data)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Skip sending notification to self
	if fromUser, ok := dataMap["from_user"].(string); ok && fromUser == userID {
		c.JSON(http.StatusOK, gin.H{
			"message": gin.H{
				"success": 200,
				"message": "Skipped sending notification to self",
			},
		})
		return
	}

	// Convert data fields to string values for FCM
	notificationData := convertToStringMap(dataMap)

	// Send notification to all user tokens
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	validTokens := make([]string, 0, len(tokens))
	for _, token := range tokens {
		message := &messaging.Message{
			Token:   token,
			Webpush: webpushConfig,
			Data:    notificationData,
		}

		logNotificationSent("user", token, message)

		response, err := messagingClient.Send(ctx, message)
		if err != nil {
			log.Printf("Failed to send notification to token %s: %v", token, err)
			continue
		}

		logNotificationResponse("token", token, response)
		validTokens = append(validTokens, token)
	}

	// Return response based on success
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

// sendNotificationToTopic sends a web push notification to a Firebase topic.
// Takes topic name, title, body and additional data from query parameters.
// Returns a JSON response with the sending result.
func sendNotificationToTopic(c *gin.Context) {
	topic := c.Query("topic_name")
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := formatProjectKey(projectName, siteName)
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

	// Validate project exists
	if err := validateProject(projectName); err != nil {
		sendErrorResponse(c, http.StatusNotFound, err.Error())
		return
	}

	// Check if topic name is empty
	if topic == "" {
		sendErrorResponse(c, http.StatusBadRequest, "topic_name is required")
		return
	}

	// Validate notification parameters
	if err := validateNotificationParams(title, body); err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Prepare web push config (pass topic for topic-specific handling)
	webpushConfig, err := prepareWebPushConfig(key, title, body, data, topic)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, fmt.Sprintf("Failed to prepare notification: %v", err))
		return
	}

	// Parse notification data
	dataMap, err := parseNotificationData(data)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	// Convert data fields to string values for FCM
	notificationData := convertToStringMap(dataMap)

	message := &messaging.Message{
		Topic:   topic,
		Webpush: webpushConfig,
		Data:    notificationData,
	}

	logNotificationSent("topic", topic, message)

	// Send the message
	ctx := context.Background()
	response, err := messagingClient.Send(ctx, message)
	if err != nil {
		sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to send notification: %v", err))
		return
	}

	logNotificationResponse("topic", topic, response)
	sendSuccessResponse(c, fmt.Sprintf("Notification sent to %s topic", topic))
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

// Add helper for consistent key formatting
func formatProjectKey(projectName, siteName string) string {
	return fmt.Sprintf("%s_%s", projectName, siteName)
}

// Add project validation helper
func validateProject(projectName string) error {
	if _, exists := config.Projects[projectName]; !exists {
		return fmt.Errorf("project %s not found", projectName)
	}
	return nil
}
