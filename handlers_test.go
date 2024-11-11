package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name           string
		setupConfig    func()
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful config retrieval",
			setupConfig: func() {
				config = Config{
					VapidPublicKey: "test-vapid-key",
					FirebaseConfig: map[string]interface{}{
						"apiKey": "test-firebase-key",
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"vapid_public_key": "test-vapid-key",
				"config": map[string]interface{}{
					"apiKey": "test-firebase-key",
				},
			},
		},
		{
			name: "missing vapid key",
			setupConfig: func() {
				config = Config{
					FirebaseConfig: map[string]interface{}{
						"apiKey": "test-firebase-key",
					},
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"exc": "VAPID public key not configured",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			tt.setupConfig()

			getConfig(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

func TestGetCredential(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test server to simulate webhook endpoint
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/valid-webhook" {
			w.Write([]byte("valid-token"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Parse test server URL to get host and port
	tsURL := ts.URL[7:] // Remove "http://" prefix

	tests := []struct {
		name           string
		request        CredentialRequest
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful credential generation",
			request: CredentialRequest{
				Endpoint:     tsURL, // Use test server URL
				Protocol:     "http",
				Token:        "valid-token",
				WebhookRoute: "/valid-webhook",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response CredentialResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.True(t, response.Success)
				assert.NotEmpty(t, response.Credentials.APIKey)
				assert.NotEmpty(t, response.Credentials.APISecret)
			},
		},
		{
			name: "missing required fields",
			request: CredentialRequest{
				Protocol: "http",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response CredentialResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.False(t, response.Success)
				assert.Equal(t, "Missing required fields", response.Message)
			},
		},
		{
			name: "invalid webhook response",
			request: CredentialRequest{
				Endpoint:     tsURL,
				Protocol:     "http",
				Token:        "invalid-token",
				WebhookRoute: "/invalid-webhook",
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response CredentialResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				assert.False(t, response.Success)
				assert.Equal(t, "Token verification failed", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			var err error
			c.Request, err = makeTestRequest(http.MethodPost, "/get-credential", tt.request)
			require.NoError(t, err)

			getCredential(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w)
		})
	}
}

func TestAPIBasicAuth(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name           string
		setupAuth      func()
		credentials    string
		expectedStatus int
	}{
		{
			name: "valid credentials",
			setupAuth: func() {
				credentials["valid-key"] = "valid-secret"
			},
			credentials:    base64.StdEncoding.EncodeToString([]byte("valid-key:valid-secret")),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing auth header",
			setupAuth:      func() {},
			credentials:    "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid credentials",
			setupAuth: func() {
				credentials["valid-key"] = "valid-secret"
			},
			credentials:    base64.StdEncoding.EncodeToString([]byte("invalid-key:invalid-secret")),
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			_, router := createTestContext(w)

			tt.setupAuth()

			// Create test endpoint with auth middleware
			router.GET("/test", apiBasicAuth(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, err := makeTestRequest(http.MethodGet, "/test", nil)
			require.NoError(t, err)
			if tt.credentials != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Basic %s", tt.credentials))
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
