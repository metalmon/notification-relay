package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Create test service account file
	testServiceAccount := `{
		"type": "service_account",
		"project_id": "test-project",
		"private_key_id": "test",
		"private_key": "test",
		"client_email": "test@test.com",
		"client_id": "test",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "test"
	}`

	err := os.MkdirAll("testdata/etc/notification-relay", 0o700)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile("testdata/etc/notification-relay/service-account.json", []byte(testServiceAccount), 0o600)
	if err != nil {
		log.Fatal(err)
	}

	// Set environment variable for tests
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/etc/notification-relay/service-account.json")
}

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
			req := httptest.NewRequest("GET", "/test", http.NoBody)
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
