package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateSecureToken(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "Generate 32 char token",
			length: 32,
		},
		{
			name:   "Generate 48 char token",
			length: 48,
		},
		{
			name:   "Generate 16 char token",
			length: 16,
		},
	}

	// Regular expression to check valid characters
	validChars := regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate token
			token := generateSecureToken(tt.length)

			// Check length
			if len(token) != tt.length {
				t.Errorf("generateSecureToken() length = %v, want %v", len(token), tt.length)
			}

			// Check that token contains only valid characters
			if !validChars.MatchString(token) {
				t.Errorf("generateSecureToken() contains invalid characters: %v", token)
			}

			// Check uniqueness - generate second token and compare
			token2 := generateSecureToken(tt.length)
			if token == token2 {
				t.Error("generateSecureToken() generated identical tokens")
			}
		})
	}
}

func TestGetCredential(t *testing.T) {
	// Switch Gin to test mode
	gin.SetMode(gin.TestMode)

	// Initialize credentials map
	credentials = make(Credentials)

	// Create a test HTTP server to emulate webhook
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/valid-webhook" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("valid-token")); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer testServer.Close()

	tests := []struct {
		name           string
		request        CredentialRequest
		expectedStatus int
		wantSuccess    bool
		wantMessage    string
	}{
		{
			name: "Valid request",
			request: CredentialRequest{
				Endpoint:     testServer.URL[7:], // Remove "http://"
				Protocol:     "http",
				Token:        "valid-token",
				WebhookRoute: "/valid-webhook",
			},
			expectedStatus: http.StatusOK,
			wantSuccess:    true,
		},
		{
			name: "Missing endpoint",
			request: CredentialRequest{
				Protocol:     "http",
				Token:        "valid-token",
				WebhookRoute: "/valid-webhook",
			},
			expectedStatus: http.StatusBadRequest,
			wantSuccess:    false,
			wantMessage:    "Missing required fields",
		},
		{
			name: "Missing token",
			request: CredentialRequest{
				Endpoint:     testServer.URL[7:],
				Protocol:     "http",
				WebhookRoute: "/valid-webhook",
			},
			expectedStatus: http.StatusBadRequest,
			wantSuccess:    false,
			wantMessage:    "Missing required fields",
		},
		{
			name: "Invalid webhook response",
			request: CredentialRequest{
				Endpoint:     testServer.URL[7:],
				Protocol:     "http",
				Token:        "invalid-token",
				WebhookRoute: "/invalid-webhook",
			},
			expectedStatus: http.StatusUnauthorized,
			wantSuccess:    false,
			wantMessage:    "Token verification failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create new Gin test context for each test
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Prepare JSON request
			jsonData, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			// Create new request with JSON data
			c.Request, err = http.NewRequest(
				http.MethodPost,
				"/api/method/notification_relay.api.auth.get_credential",
				bytes.NewBuffer(jsonData),
			)
			assert.NoError(t, err)
			c.Request.Header.Set("Content-Type", "application/json")

			// Call the tested function
			getCredential(c)

			// Check response status
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Parse response
			var response CredentialResponse
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Check operation success
			assert.Equal(t, tt.wantSuccess, response.Success)

			// If error message is expected, check it
			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, response.Message)
			}

			// For successful request, check presence of credentials
			if tt.wantSuccess {
				assert.NotNil(t, response.Credentials)
				assert.NotEmpty(t, response.Credentials.APIKey)
				assert.NotEmpty(t, response.Credentials.APISecret)
			}
		})
	}
}

func TestAPIBasicAuth(t *testing.T) {
	// Switch Gin to test mode
	gin.SetMode(gin.TestMode)

	// Initialize test credentials
	credentials = make(Credentials)
	credentials["test-key"] = "test-secret"

	tests := []struct {
		name            string
		apiKey          string
		apiSecret       string
		expectedStatus  int
		checkAuthHeader bool // Add flag for checking header
	}{
		{
			name:            "Valid credentials",
			apiKey:          "test-key",
			apiSecret:       "test-secret",
			expectedStatus:  200,
			checkAuthHeader: false,
		},
		{
			name:            "Invalid key",
			apiKey:          "wrong-key",
			apiSecret:       "test-secret",
			expectedStatus:  401,
			checkAuthHeader: false,
		},
		{
			name:            "Invalid secret",
			apiKey:          "test-key",
			apiSecret:       "wrong-secret",
			expectedStatus:  401,
			checkAuthHeader: false,
		},
		{
			name:            "Missing credentials",
			apiKey:          "",
			apiSecret:       "",
			expectedStatus:  401,
			checkAuthHeader: true, // Check header only for missing credentials
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test router
			router := gin.New()

			// Add middleware and test handler
			router.Use(apiBasicAuth())
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Create test request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", http.NoBody)

			// Add Basic Auth header if there are credentials
			if tt.apiKey != "" || tt.apiSecret != "" {
				req.SetBasicAuth(tt.apiKey, tt.apiSecret)
			}

			// Perform request
			router.ServeHTTP(w, req)

			// Check response status
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Check WWW-Authenticate header only if necessary
			if tt.checkAuthHeader {
				assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Basic realm=")
			}
		})
	}
}

func TestValidateNotificationParams(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		body      string
		wantError bool
	}{
		{
			name:      "Valid params",
			title:     "Test Title",
			body:      "Test Body",
			wantError: false,
		},
		{
			name:      "Empty title",
			title:     "",
			body:      "Test Body",
			wantError: true,
		},
		{
			name:      "Empty body",
			title:     "Test Title",
			body:      "",
			wantError: true,
		},
		{
			name:      "Both empty",
			title:     "",
			body:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNotificationParams(tt.title, tt.body)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddIconToConfig(t *testing.T) {
	// Initialize test icons
	icons = make(map[string]string)
	testKey := "test-project_test-site"
	icons[testKey] = "/path/to/icon.png"

	tests := []struct {
		name   string
		key    string
		config *messaging.WebpushConfig
		want   string
	}{
		{
			name:   "Add icon to empty config",
			key:    testKey,
			config: &messaging.WebpushConfig{},
			want:   "/path/to/icon.png",
		},
		{
			name: "Add icon to config with existing data",
			key:  testKey,
			config: &messaging.WebpushConfig{
				Data: map[string]string{
					"existing": "data",
				},
			},
			want: "/path/to/icon.png",
		},
		{
			name:   "No icon for non-existent project",
			key:    "non-existent",
			config: &messaging.WebpushConfig{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addIconToConfig(tt.key, tt.config)

			if tt.want == "" {
				// For project without icon, check that icon is not added
				if tt.config.Data != nil {
					assert.Empty(t, tt.config.Data["icon"])
				}
			} else {
				// For project with icon, check that it is added correctly
				assert.NotNil(t, tt.config.Data)
				assert.Equal(t, tt.want, tt.config.Data["icon"])
			}
		})
	}
}
