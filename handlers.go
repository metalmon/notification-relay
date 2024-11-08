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

// CredentialRequest represents a request for API credentials
type CredentialRequest struct {
	Endpoint     string `json:"endpoint"`
	Protocol     string `json:"protocol"`
	Port         string `json:"port"`
	Token        string `json:"token"`
	WebhookRoute string `json:"webhook_route"`
}

// CredentialResponse represents an API credentials response
type CredentialResponse struct {
	Success     bool               `json:"success"`
	Message     string             `json:"message,omitempty"`
	Credentials *CredentialDetails `json:"credentials,omitempty"`
}

// CredentialDetails contains generated API credentials
type CredentialDetails struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// Credentials represents a map of API credentials
type Credentials map[string]string

// Response represents the standard API response structure
type Response struct {
	Message interface{} `json:"message,omitempty"` // For successful responses
	Data    interface{} `json:"data,omitempty"`    // For data responses
	Exc     string      `json:"exc,omitempty"`     // For critical errors
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

var (
	credentials Credentials
)

// Add initialization function that will be called after configPath is set
func initCredentials() {
	// Load credentials from file
	ensureFileExists(CredentialsJSON, make(Credentials))
	if err := loadJSON(CredentialsJSON, &credentials); err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
}

// getCredential handles API credential requests by validating the request,
// verifying the provided token by making a request to the site's webhook,
// and returning API credentials if verification is successful.
// It expects a CredentialRequest with endpoint, protocol, port, token and webhook route.
// Returns a CredentialResponse with success status and either credentials or error message.
func getCredential(c *gin.Context) {
	var req CredentialRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CredentialResponse{
			Success: false,
			Message: "Invalid request format",
		})
		return
	}

	// Validate the request
	if req.Endpoint == "" || req.Token == "" {
		c.JSON(http.StatusBadRequest, CredentialResponse{
			Success: false,
			Message: "Missing required fields",
		})
		return
	}

	// Verify token by making request to the site's webhook
	webhookURL := fmt.Sprintf("%s://%s%s%s",
		req.Protocol,
		req.Endpoint,
		func() string {
			if req.Port != "" {
				return ":" + req.Port
			}
			return ""
		}(),
		req.WebhookRoute,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(webhookURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, CredentialResponse{
			Success: false,
			Message: "Failed to verify token",
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

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, CredentialResponse{
			Success: false,
			Message: "Token verification failed",
		})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != req.Token {
		c.JSON(http.StatusUnauthorized, CredentialResponse{
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
		c.JSON(http.StatusInternalServerError, CredentialResponse{
			Success: false,
			Message: "Failed to save credentials",
		})
		return
	}

	c.JSON(http.StatusOK, CredentialResponse{
		Success: true,
		Credentials: &CredentialDetails{
			APIKey:    apiKey,
			APISecret: apiSecret,
		},
	})
}

// generateSecureToken generates a cryptographically secure random token of the specified length
// using characters from the charset (a-z, A-Z, 0-9). Returns the generated token as a string.
func generateSecureToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// apiBasicAuth middleware для проверки Basic Auth
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

	if tokens, exists := userDeviceMap[key][userID]; exists && len(tokens) > 0 {
		ctx := context.Background()
		client, err := fbApp.Messaging(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Exc: fmt.Sprintf("Failed to initialize messaging client: %v", err),
			})
			return
		}

		_, err = client.SubscribeToTopic(ctx, tokens, topicName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Exc: fmt.Sprintf("Failed to subscribe to topic: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": true,
				"message": "User subscribed",
			},
		})
		return
	}

	// Даже ошибка валидации возвращается с 200
	c.JSON(http.StatusOK, Response{
		Message: map[string]interface{}{
			"success": false,
			"message": userID + " not subscribed to push notifications",
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
		ctx := context.Background()
		client, err := fbApp.Messaging(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Exc: fmt.Sprintf("Failed to initialize messaging client: %v", err),
			})
			return
		}

		_, err = client.UnsubscribeFromTopic(ctx, tokens, topicName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Exc: fmt.Sprintf("Failed to unsubscribe from topic: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("User %s unsubscribed from %s topic", userID, topicName),
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Message: map[string]interface{}{
			"success": false,
			"message": userID + " not subscribed to push notifications",
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
			if token != fcmToken {
				continue
			}
			c.JSON(http.StatusOK, Response{
				Message: map[string]interface{}{
					"success": true,
					"message": "User Token duplicate found",
				},
			})
			return
		}
		userDeviceMap[key][userID] = append(tokens, fcmToken)
	} else {
		userDeviceMap[key][userID] = []string{fcmToken}
	}

	err := saveJSON(UserDeviceMapJSON, userDeviceMap)
	if err != nil {
		excBytes, _ := json.Marshal([]string{fmt.Sprintf("Failed to save user device map: %v", err)})
		c.JSON(http.StatusInternalServerError, Response{
			Exc: string(excBytes),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Message: map[string]interface{}{
			"success": true,
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
			if token != fcmToken {
				continue
			}
			// Нашли токен, удаляем его
			userDeviceMap[key][userID] = append(tokens[:i], tokens[i+1:]...)
			err := saveJSON(UserDeviceMapJSON, userDeviceMap)
			if err != nil {
				excBytes, _ := json.Marshal([]string{"Failed to save user device map"})
				c.JSON(http.StatusInternalServerError, Response{
					Exc: string(excBytes),
				})
				return
			}
			c.JSON(http.StatusOK, Response{
				Message: map[string]interface{}{
					"success": true,
					"message": "User Token removed",
				},
			})
			return
		}
	}

	// Если токен не найден, все равно возвращаем успешный результат
	c.JSON(http.StatusOK, Response{
		Message: map[string]interface{}{
			"success": true,
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

// applyDecorations применяет декорации к заголовку уведомления
func applyDecorations(key, title string) string {
	if projectDecorations, exists := decorations[key]; exists {
		for _, decoration := range projectDecorations {
			if matched, _ := regexp.MatchString(decoration.Pattern, title); matched {
				return strings.Replace(decoration.Template, "{title}", title, 1)
			}
		}
	}
	return title
}

// addIconToConfig добавляет иконку проекта в конфигурацию
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
	// Get request parameters
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

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

	// Get user's tokens
	tokens, err := getUserTokens(key, userID)
	if err != nil {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		})
		return
	}

	webpushConfig, err := prepareWebPushConfig(key, title, body, data)
	if err != nil {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		})
		return
	}

	// Send notification to all user tokens
	var lastError error
	for _, token := range tokens {
		message := &messaging.Message{
			Token:   token,
			Webpush: webpushConfig,
		}

		ctx := context.Background()
		client, err := fbApp.Messaging(ctx)
		if err != nil {
			lastError = err
			continue
		}

		_, err = client.Send(ctx, message)
		if err != nil {
			lastError = err
			continue
		}
	}

	// Check if there were any errors during sending
	if lastError != nil {
		excBytes, _ := json.Marshal([]string{fmt.Sprintf("Failed to send notification: %v", lastError)})
		c.JSON(http.StatusInternalServerError, Response{
			Exc: string(excBytes),
		})
		return
	}

	// Return successful response only after all notifications are sent
	c.JSON(http.StatusOK, Response{
		Message: map[string]interface{}{
			"success": true,
			"message": "Notification sent",
		},
	})
}

// applyTopicDecorations applies decorations to the notification title based on topic
func applyTopicDecorations(topic, title string) string {
	if decoration, exists := topicDecorations[topic]; exists {
		if matched, _ := regexp.MatchString(decoration.Pattern, title); matched {
			return strings.Replace(decoration.Template, "{title}", title, 1)
		}
	}
	return title
}

// prepareTopicWebPushConfig creates a web push notification configuration for a topic
func prepareTopicWebPushConfig(topic, title, body, data string) (*messaging.WebpushConfig, error) {
	// Применяем декорации к заголовку на основе топика
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

	webpushConfig, err := prepareTopicWebPushConfig(topic, title, body, data)
	if err != nil {
		c.JSON(http.StatusOK, Response{
			Message: map[string]interface{}{
				"success": false,
				"message": err.Error(),
			},
		})
		return
	}

	message := &messaging.Message{
		Topic:   topic,
		Webpush: webpushConfig,
	}

	ctx := context.Background()
	client, err := fbApp.Messaging(ctx)
	if err != nil {
		excBytes, _ := json.Marshal([]string{fmt.Sprintf("Failed to initialize messaging client: %v", err)})
		c.JSON(http.StatusInternalServerError, Response{
			Exc: string(excBytes),
		})
		return
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		excBytes, _ := json.Marshal([]string{fmt.Sprintf("Failed to send notification: %v", err)})
		c.JSON(http.StatusInternalServerError, Response{
			Exc: string(excBytes),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Message: map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Notification sent to %s topic", topic),
		},
	})
}
