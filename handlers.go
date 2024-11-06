package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
)

type CredentialRequest struct {
	Endpoint      string `json:"endpoint"`
	Protocol      string `json:"protocol"`
	Port          string `json:"port"`
	Token         string `json:"token"`
	WebhookRoute  string `json:"webhook_route"`
}

type CredentialResponse struct {
	Success     bool                   `json:"success"`
	Message     string                `json:"message,omitempty"`
	Credentials *CredentialDetails    `json:"credentials,omitempty"`
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
	ensureFileExists("credentials.json", make(Credentials))
	loadJSON("credentials.json", &credentials)
}

func getConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"vapid_public_key": config.VapidPublicKey,
		"config":          config.FirebaseConfig,
	})
}

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

		response, err := client.SubscribeToTopic(ctx, tokens, topicName)
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
		req.Port != "" ? ":" + req.Port : "",
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
	err = saveJSON("credentials.json", credentials)
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

// Update the basicAuth middleware to use credentials from file
func basicAuth() gin.HandlerFunc {
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

// Continue with other handlers... 