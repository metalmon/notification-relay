package main

import (
	"context"
	"net/http"
	"regexp"

	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
)

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

// Continue with other handlers... 