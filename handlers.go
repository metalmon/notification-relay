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
	"os"
	"regexp"
	"strings"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
)

type CredentialRequest struct {
	Endpoint     string `json:"endpoint"`
	Protocol     string `json:"protocol"`
	Port         string `json:"port"`
	Token        string `json:"token"`
	WebhookRoute string `json:"webhook_route"`
}

type CredentialResponse struct {
	Success     bool               `json:"success"`
	Message     string             `json:"message,omitempty"`
	Credentials *CredentialDetails `json:"credentials,omitempty"`
}

type CredentialDetails struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

type Credentials map[string]string

var (
	credentials Credentials
)

func init() {
	// Load credentials from file
	ensureFileExists(CredentialsJSON, make(Credentials))
	loadJSON(CredentialsJSON, &credentials)
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
	defer resp.Body.Close()

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

	c.JSON(http.StatusBadRequest, Response{
		Error: &ErrorResponse{
			StatusCode: 404,
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

	c.JSON(http.StatusBadRequest, Response{
		Error: &ErrorResponse{
			StatusCode: 404,
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

	if userDeviceMap[key] == nil {
		userDeviceMap[key] = make(map[string][]string)
	}

	if tokens, exists := userDeviceMap[key][userID]; exists {
		for _, token := range tokens {
			if token == fcmToken {
				c.JSON(http.StatusOK, Response{
					Message: &SuccessResponse{
						Success: 200,
						Message: "User Token duplicate found",
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
			if token == fcmToken {
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
	}

	c.JSON(http.StatusOK, Response{
		Message: &SuccessResponse{
			Success: 200,
			Message: "User Token removed",
		},
	})
}

func sendNotificationToUser(c *gin.Context) {
	projectName := c.Query("project_name")
	siteName := c.Query("site_name")
	key := projectName + "_" + siteName
	userID := c.Query("user_id")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

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

	notificationIcon, _ := dataMap["notification_icon"].(string)
	if notificationIcon == "" {
		if icon, exists := icons[key]; exists {
			if _, err := os.Stat(icon); err == nil {
				notificationIcon = icon
			}
		}
	}

	// Check title against decoration patterns if project exists
	if projectDecorations, exists := decorations[key]; exists {
		for _, config := range projectDecorations {
			matched, err := regexp.MatchString(config.Pattern, title)
			if err == nil && matched {
				title = strings.Replace(config.Template, "{title}", title, 1)
				break
			}
		}
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

		message := &messaging.MulticastMessage{
			Webpush: &messaging.WebpushConfig{
				Notification: &messaging.WebpushNotification{
					Title: title,
					Body:  body,
					Icon:  notificationIcon,
				},
				FCMOptions: &messaging.WebpushFCMOptions{
					Link: dataMap["click_action"].(string),
				},
			},
			Tokens: tokens,
		}

		response, err := client.SendMulticast(ctx, message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{
				Error: &ErrorResponse{
					StatusCode: http.StatusInternalServerError,
					Message:    "Failed to send notification",
				},
			})
			return
		}

		// Handle failures and update user-device map
		if response.FailureCount > 0 {
			for i, resp := range response.Responses {
				if !resp.Success {
					// Remove failed token
					userDeviceMap[key][userID] = append(tokens[:i], tokens[i+1:]...)
				}
			}
			// Save updated map
			if err := saveJSON(UserDeviceMapJSON, userDeviceMap); err != nil {
				log.Printf("Failed to save updated user device map: %v", err)
			}
		}

		c.JSON(http.StatusOK, Response{
			Message: &SuccessResponse{
				Success: 200,
				Message: fmt.Sprintf("%d Notification sent to %s user", response.SuccessCount, userID),
			},
		})
		return
	}

	c.JSON(http.StatusBadRequest, Response{
		Error: &ErrorResponse{
			StatusCode: 404,
			Message:    userID + " not subscribed to push notifications",
		},
	})
}

func sendNotificationToTopic(c *gin.Context) {
	topic := c.Query("topic_name")
	title := c.Query("title")
	body := c.Query("body")
	data := c.Query("data")

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

	notificationIcon, _ := dataMap["notification_icon"].(string)
	if notificationIcon == "" {
		if icon, exists := icons[topic]; exists {
			notificationIcon = icon
		} else {
			notificationIcon = "" // Default empty icon if not found
		}
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

	message := &messaging.Message{
		Webpush: &messaging.WebpushConfig{
			Notification: &messaging.WebpushNotification{
				Title: title,
				Body:  body,
				Icon:  notificationIcon,
			},
			FCMOptions: &messaging.WebpushFCMOptions{
				Link: dataMap["click_action"].(string),
			},
		},
		Topic: topic,
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
