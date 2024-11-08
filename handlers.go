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

	firebase "firebase.google.com/go/v4"
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

var (
	credentials     Credentials
	messagingClient *messaging.Client
)

// Add initialization function that will be called after configPath is set
func initCredentials() {
	// Load credentials from file
	ensureFileExists(CredentialsJSON, make(Credentials))
	if err := loadJSON(CredentialsJSON, &credentials); err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
}

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
		c.JSON(http.StatusBadRequest, CredentialResponse{
			Success: false,
			Message: "Token verification failed",
		})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != req.Token {
		c.JSON(http.StatusBadRequest, CredentialResponse{
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

func generateSecureToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// Update the basicAuth middleware to use credentials from file

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
				Error: &ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    "Failed to initialize messaging client",
				},
			})
			return
		}

		_, err = client.SubscribeToTopic(ctx, tokens, topicName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Error: &ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusOK, Response{
			Message: &SuccessResponse{
				Success: 200,
				Message: "User subscribed",
			},
		})
		return
	}

	c.JSON(http.StatusNotFound, Response{
		Error: &ErrorResponse{
			StatusCode: http.StatusNotFound,
			Message:    userID + " not subscribed to push notifications",
		},
	})
}

func unsubscribeFromTopic(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	topicName := c.Query("topic_name")

	// Check if topic name is empty
	if topicName == "" {
		c.JSON(http.StatusBadRequest, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "topic_name is required",
			},
		})
		return
	}

	if tokens, exists := userDeviceMap[key][userID]; exists && len(tokens) > 0 {
		ctx := context.Background()
		client, err := fbApp.Messaging(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Error: &ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    "Failed to initialize messaging client",
				},
			})
			return
		}

		_, err = client.UnsubscribeFromTopic(ctx, tokens, topicName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Error: &ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusOK, Response{
			Message: &SuccessResponse{
				Success: 200,
				Message: fmt.Sprintf("User %s unsubscribed from %s topic", userID, topicName),
			},
		})
		return
	}

	// Changed to match HTTP status with error status code
	c.JSON(http.StatusBadRequest, Response{
		Error: &ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Message:    userID + " not subscribed to push notifications",
		},
	})
}

func addToken(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	fcmToken := c.Query("fcm_token")

	// Check if token is empty
	if fcmToken == "" {
		c.JSON(http.StatusBadRequest, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "FCM token is required",
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
				Message: &SuccessResponse{
					Success: 200,
					Message: "User Token duplicate found",
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
		c.JSON(http.StatusInternalServerError, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to save user device map",
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Message: &SuccessResponse{
			Success: 200,
			Message: "User Token added",
		},
	})
}

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
				c.JSON(http.StatusInternalServerError, Response{
					Error: &ErrorResponse{
						StatusCode: http.StatusInternalServerError,
						Message:    "Failed to save user device map",
					},
				})
				return
			}
			c.JSON(http.StatusOK, Response{
				Message: &SuccessResponse{
					Success: 200,
					Message: "User Token removed",
				},
			})
			return
		}
	}

	// Если токен не найден, все равно возвращаем успешный результат
	c.JSON(http.StatusOK, Response{
		Message: &SuccessResponse{
			Success: 200,
			Message: "User Token removed",
		},
	})
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

// getUserTokens retrieves the user's tokens
func getUserTokens(key, userID string) ([]string, error) {
	tokens, exists := userDeviceMap[key][userID]
	if !exists || len(tokens) == 0 {
		return nil, fmt.Errorf("user %s not subscribed to push notifications", userID)
	}
	return tokens, nil
}

// prepareNotification prepares a notification with decorations
func prepareNotification(key, title, body, data string) (*messaging.Message, error) {
	// Apply decorations to the title if they exist
	if projectDecorations, exists := decorations[key]; exists {
		for _, decoration := range projectDecorations {
			if matched, _ := regexp.MatchString(decoration.Pattern, title); matched {
				title = strings.Replace(decoration.Template, "{title}", title, 1)
				break
			}
		}
	}

	// Prepare notification data
	notification := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
	}

	// Add additional data if it exists
	if data != "" {
		notification.Data = map[string]string{"data": data}
	}

	// Add project icon if it exists
	if iconPath, exists := icons[key]; exists {
		if notification.Data == nil {
			notification.Data = make(map[string]string)
		}
		notification.Data["icon"] = iconPath
	}

	return notification, nil
}

// initMessaging initializes the Firebase Messaging client
func initMessaging(app *firebase.App) error {
	var err error
	ctx := context.Background()
	messagingClient, err = app.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("error getting Messaging client: %v", err)
	}
	return nil
}

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
		c.JSON(http.StatusBadRequest, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    err.Error(),
			},
		})
		return
	}

	// Get user's tokens
	tokens, err := getUserTokens(key, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    err.Error(),
			},
		})
		return
	}

	// Prepare notification
	notification, err := prepareNotification(key, title, body, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to prepare notification",
			},
		})
		return
	}

	// Send notification to all user tokens
	for _, token := range tokens {
		notification.Token = token
		_, err := messagingClient.Send(context.Background(), notification)
		if err != nil {
			log.Printf("Failed to send notification to token %s: %v", token, err)
		}
	}

	c.JSON(http.StatusOK, Response{
		Message: &SuccessResponse{
			Success: 200,
			Message: "Notification sent",
		},
	})
}

func sendNotificationToTopic(c *gin.Context) {
	topic := c.Query("topic_name")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

	// Check if topic name is empty
	if topic == "" {
		c.JSON(http.StatusBadRequest, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "topic_name is required",
			},
		})
		return
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Message:    "Invalid data format",
			},
		})
		return
	}

	ctx := context.Background()
	client, err := fbApp.Messaging(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to initialize messaging client",
			},
		})
		return
	}

	webpushConfig := &messaging.WebpushConfig{
		Notification: &messaging.WebpushNotification{
			Title: title,
			Body:  body,
		},
	}

	// Add click_action only if it exists
	if clickAction, ok := dataMap["click_action"].(string); ok {
		webpushConfig.FCMOptions = &messaging.WebpushFCMOptions{
			Link: clickAction,
		}
	}

	message := &messaging.Message{
		Topic:   topic,
		Webpush: webpushConfig,
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Error: &ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Message:    "Failed to send notification",
			},
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Message: &SuccessResponse{
			Success: 200,
			Message: fmt.Sprintf("Notification sent to %s topic", topic),
		},
	})
}

// Continue with other handlers...

// Response represents the standard API response structure
type Response struct {
	Message *SuccessResponse `json:"message,omitempty"`
	Error   *ErrorResponse   `json:"error,omitempty"`
}

// SuccessResponse represents a successful API response message
type SuccessResponse struct {
	Success int    `json:"success"`
	Message string `json:"message"`
}

// ErrorResponse represents an error API response message
type ErrorResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

// Decoration represents a notification title decoration rule
type Decoration struct {
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
}

// Добавим функцию apiBasicAuth
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
