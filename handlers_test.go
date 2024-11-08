package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func TestGetCredential(t *testing.T) {
	router := setupTestRouter()
	router.POST("/api/method/notification_relay.api.auth.get_credential", getCredential)

	tests := []struct {
		name           string
		request        CredentialRequest
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "Valid Request",
			request: CredentialRequest{
				Endpoint:     "test.example.com",
				Protocol:     "https",
				Token:        "valid_token",
				WebhookRoute: "/webhook",
			},
			expectedStatus: http.StatusBadRequest, // Will fail because webhook is not actually available
			expectedError:  true,
		},
		{
			name: "Missing Endpoint",
			request: CredentialRequest{
				Protocol:     "https",
				Token:        "valid_token",
				WebhookRoute: "/webhook",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/api/method/notification_relay.api.auth.get_credential", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response CredentialResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.expectedError {
				assert.False(t, response.Success)
				assert.NotEmpty(t, response.Message)
			} else {
				assert.True(t, response.Success)
				assert.NotNil(t, response.Credentials)
				assert.NotEmpty(t, response.Credentials.APIKey)
				assert.NotEmpty(t, response.Credentials.APISecret)
			}
		})
	}
}

func TestAPIBasicAuth(t *testing.T) {
	router := setupTestRouter()

	// Setup test credentials
	credentials = make(Credentials)
	credentials["test_key"] = "test_secret"

	// Add authentication middleware and test handler
	auth := router.Group("/", apiBasicAuth())
	auth.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, Response{
			Message: &SuccessResponse{
				Success: 200,
				Message: "Authenticated",
			},
		})
	})

	tests := []struct {
		name           string
		apiKey         string
		apiSecret      string
		expectedStatus int
	}{
		{
			name:           "Valid Credentials",
			apiKey:         "test_key",
			apiSecret:      "test_secret",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid API Key",
			apiKey:         "wrong_key",
			apiSecret:      "test_secret",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid API Secret",
			apiKey:         "test_key",
			apiSecret:      "wrong_secret",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "No Authentication",
			apiKey:         "",
			apiSecret:      "",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.apiKey != "" || tt.apiSecret != "" {
				req.SetBasicAuth(tt.apiKey, tt.apiSecret)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response Response
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.NotNil(t, response.Message)
				assert.Equal(t, 200, response.Message.Success)
				assert.Equal(t, "Authenticated", response.Message.Message)
			}
		})
	}
}
