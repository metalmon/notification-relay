package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/your-username/notification-relay/mocks"

	"github.com/stretchr/testify/assert"

	"firebase.google.com/go/v4/messaging"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errInvalidToken = fmt.Errorf("invalid registration token")

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
					Projects: map[string]ProjectConfig{
						"test_project": {
							VapidPublicKey: "test-vapid-key",
							FirebaseConfig: FirebaseConfig{
								ApiKey: "test-firebase-key",
							},
						},
					},
					TrustedProxies: "127.0.0.1",
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"vapid_public_key": "test-vapid-key",
					"config": map[string]interface{}{
						"apiKey": "test-firebase-key",
					},
				},
			},
		},
		{
			name: "missing vapid key",
			setupConfig: func() {
				config = Config{
					Projects: map[string]ProjectConfig{
						"test_project": {
							VapidPublicKey: "",
							FirebaseConfig: FirebaseConfig{
								ApiKey: "test-firebase-key",
							},
						},
					},
					TrustedProxies: "127.0.0.1",
				}
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"exc": map[string]interface{}{
					"status_code": 400,
					"message":     "VAPID public key not configured",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			tt.setupConfig()

			// Setup request with project_name query parameter
			c.Request, _ = http.NewRequest("GET", "/api/method/notification_relay.api.get_config?project_name=test_project", http.NoBody)

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
			if _, err := w.Write([]byte("valid-token")); err != nil {
				t.Fatal(err)
			}
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

func TestSendNotificationToUser(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mocks.MockFirebaseMessagingClient{}
	messagingClient = mockClient

	// Set up test user and device
	key := "test_project_test_site"
	userID := "test_user"
	token := "test_token"
	userDeviceMap[key] = map[string][]string{
		userID: {token},
	}

	// Set up test decorations
	decorations[key] = map[string]Decoration{
		"alert": {
			Pattern:  "^Alert:",
			Template: "🚨 {title}",
		},
	}

	// Set up test icons
	icons[key] = "/path/to/icon.png"

	tests := []struct {
		name           string
		setupMock      func()
		queryParams    map[string]string
		expectedStatus int
		expectedBody   map[string]interface{}
		checkMessage   func(*testing.T, *messaging.Message)
	}{
		{
			name: "successful notification with decoration",
			setupMock: func() {
				mockClient.On("Send",
					mock.Anything,
					mock.MatchedBy(func(msg *messaging.Message) bool {
						return msg.Token == token &&
							msg.Webpush.Notification.Title == "🚨 Alert: Test Message" &&
							msg.Webpush.Notification.Body == "Test Body" &&
							msg.Webpush.Data["icon"] == "/path/to/icon.png"
					}),
				).Return("message_id", nil)
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      userID,
				"title":        "Alert: Test Message",
				"body":         "Test Body",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "Notification sent",
				},
			},
		},
		{
			name: "successful notification with click action",
			setupMock: func() {
				mockClient.On("Send",
					mock.Anything,
					mock.MatchedBy(func(msg *messaging.Message) bool {
						return msg.Token == token &&
							msg.Webpush.FCMOptions.Link == "https://example.com"
					}),
				).Return("message_id", nil)
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      userID,
				"title":        "Test Title",
				"body":         "Test Body",
				"data":         `{"click_action": "https://example.com"}`,
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "Notification sent",
				},
			},
		},
		{
			name: "invalid token removed",
			setupMock: func() {
				mockClient.On("Send",
					mock.Anything,
					mock.Anything,
				).Return("", errInvalidToken)
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      userID,
				"title":        "Test Title",
				"body":         "Test Body",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": false,
					"message": "No valid tokens found",
				},
			},
			checkMessage: func(t *testing.T, msg *messaging.Message) {
				// Verify token was removed
				tokens := userDeviceMap[key][userID]
				assert.Empty(t, tokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			// Setup mock
			mockClient.ExpectedCalls = nil // Clear previous mock expectations
			tt.setupMock()

			// Setup request with query parameters
			req, err := http.NewRequest(http.MethodPost, "/send", http.NoBody)
			require.NoError(t, err)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			sendNotificationToUser(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)

			mockClient.AssertExpectations(t)
		})
	}
}

func TestValidateNotificationParams(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		body        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid params",
			title:       "Test Title",
			body:        "Test Body",
			expectError: false,
		},
		{
			name:        "missing title",
			title:       "",
			body:        "Test Body",
			expectError: true,
			errorMsg:    "title is required",
		},
		{
			name:        "missing body",
			title:       "Test Title",
			body:        "",
			expectError: true,
			errorMsg:    "body is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNotificationParams(tt.title, tt.body)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPrepareWebPushConfig(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name           string
		key            string
		title          string
		body           string
		data           string
		setupData      func()
		expectError    bool
		validateConfig func(*testing.T, *messaging.WebpushConfig)
	}{
		{
			name:  "basic config",
			key:   "test_project",
			title: "Test Title",
			body:  "Test Body",
			data:  "",
			validateConfig: func(t *testing.T, config *messaging.WebpushConfig) {
				assert.Equal(t, "Test Title", config.Notification.Title)
				assert.Equal(t, "Test Body", config.Notification.Body)
			},
		},
		{
			name:  "with click action",
			key:   "test_project",
			title: "Test Title",
			body:  "Test Body",
			data:  `{"click_action": "https://example.com"}`,
			validateConfig: func(t *testing.T, config *messaging.WebpushConfig) {
				assert.Equal(t, "https://example.com", config.FCMOptions.Link)
			},
		},
		{
			name:        "invalid data json",
			key:         "test_project",
			title:       "Test Title",
			body:        "Test Body",
			data:        "invalid json",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupData != nil {
				tt.setupData()
			}

			config, err := prepareWebPushConfig(tt.key, tt.title, tt.body, tt.data, "")

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, config)
			if tt.validateConfig != nil {
				tt.validateConfig(t, config)
			}
		})
	}
}

func TestSubscribeToTopic(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mocks.MockFirebaseMessagingClient{}
	messagingClient = mockClient

	tests := []struct {
		name           string
		setupMock      func()
		queryParams    map[string]string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful subscription",
			setupMock: func() {
				mockClient.On("SubscribeToTopic",
					mock.Anything,
					[]string{"test_token"},
					"test_topic",
				).Return(&messaging.TopicManagementResponse{SuccessCount: 1, FailureCount: 0}, nil)
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
				"topic_name":   "test_topic",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": 200,
					"message": "User subscribed to topic test_topic. Success: 1, Failures: 0",
				},
			},
		},
		{
			name:      "missing topic name",
			setupMock: func() {},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": false,
					"message": "topic_name is required",
				},
			},
		},
		{
			name:      "user not subscribed",
			setupMock: func() {},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "nonexistent_user",
				"topic_name":   "test_topic",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": false,
					"message": "nonexistent_user not subscribed to push notifications",
				},
			},
		},
		{
			name: "firebase client error",
			setupMock: func() {
				mockClient.On("SubscribeToTopic",
					mock.Anything,
					[]string{"test_token"},
					"test_topic",
				).Return(&messaging.TopicManagementResponse{}, fmt.Errorf("firebase error"))
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
				"topic_name":   "test_topic",
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"exc": "Failed to subscribe to topic: firebase error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			// Setup mock
			mockClient.ExpectedCalls = nil // Clear previous mock expectations
			tt.setupMock()

			// Setup test environment
			config = Config{
				Projects: map[string]ProjectConfig{
					"test_project": {
						VapidPublicKey: "test-vapid-key",
						FirebaseConfig: FirebaseConfig{
							ApiKey: "test-firebase-key",
						},
					},
				},
				TrustedProxies: "127.0.0.1",
			}

			key := fmt.Sprintf("%s_%s", tt.queryParams["project_name"], tt.queryParams["site_name"])
			userID := tt.queryParams["user_id"]
			userDeviceMap[key] = map[string][]string{
				userID: {"test_token"},
			}

			// Setup request with query parameters
			req, err := http.NewRequest(http.MethodPost, "/subscribe", http.NoBody)
			require.NoError(t, err)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			subscribeToTopic(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)

			mockClient.AssertExpectations(t)
		})
	}
}

func TestUnsubscribeFromTopic(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mocks.MockFirebaseMessagingClient{}
	messagingClient = mockClient

	// Set up test user and device
	key := "test_project_test_site"
	userID := "test_user"
	token := "test_token"
	userDeviceMap[key] = map[string][]string{
		userID: {token},
	}

	tests := []struct {
		name           string
		setupMock      func()
		queryParams    map[string]string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful unsubscription",
			setupMock: func() {
				mockClient.On("UnsubscribeFromTopic",
					mock.Anything,
					[]string{token},
					"test_topic",
				).Return(&messaging.TopicManagementResponse{}, nil)
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      userID,
				"topic_name":   "test_topic",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "User test_user unsubscribed from test_topic topic",
				},
			},
		},
		{
			name: "missing topic name",
			setupMock: func() {
				// No mock setup needed - function should return early
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      userID,
				// topic_name intentionally omitted
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": false,
					"message": "topic_name is required",
				},
			},
		},
		{
			name: "user not subscribed",
			setupMock: func() {
				// No mock setup needed - function should return early
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "nonexistent_user",
				"topic_name":   "test_topic",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": false,
					"message": "nonexistent_user not subscribed to push notifications",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			// Setup mock
			mockClient.ExpectedCalls = nil // Clear previous mock expectations
			tt.setupMock()

			// Setup request with query parameters
			req, err := http.NewRequest(http.MethodPost, "/unsubscribe", http.NoBody)
			require.NoError(t, err)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			unsubscribeFromTopic(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)

			mockClient.AssertExpectations(t)
		})
	}
}

func TestApplyDecorations(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		title            string
		setupDecorations func()
		expected         string
	}{
		{
			name:  "no decorations",
			key:   "test_project",
			title: "Test Title",
			setupDecorations: func() {
				decorations = make(map[string]map[string]Decoration)
			},
			expected: "Test Title",
		},
		{
			name:  "matching decoration",
			key:   "test_project",
			title: "Alert: Test Message",
			setupDecorations: func() {
				decorations = map[string]map[string]Decoration{
					"test_project": {
						"alert": {
							Pattern:  "^Alert:",
							Template: "🚨 {title}",
						},
					},
				}
			},
			expected: "🚨 Alert: Test Message",
		},
		{
			name:  "non-matching decoration",
			key:   "test_project",
			title: "Normal Message",
			setupDecorations: func() {
				decorations = map[string]map[string]Decoration{
					"test_project": {
						"alert": {
							Pattern:  "^Alert:",
							Template: "🚨 {title}",
						},
					},
				}
			},
			expected: "Normal Message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupDecorations()
			result := applyDecorations(tt.key, tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddIconToConfig(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupIcons     func()
		validateConfig func(*testing.T, *messaging.WebpushConfig)
	}{
		{
			name: "add icon when available",
			key:  "test_project",
			setupIcons: func() {
				icons = map[string]string{
					"test_project": "/path/to/icon.png",
				}
			},
			validateConfig: func(t *testing.T, config *messaging.WebpushConfig) {
				assert.Equal(t, "/path/to/icon.png", config.Data["icon"])
			},
		},
		{
			name: "no icon available",
			key:  "test_project",
			setupIcons: func() {
				icons = make(map[string]string)
			},
			validateConfig: func(t *testing.T, config *messaging.WebpushConfig) {
				assert.Nil(t, config.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupIcons()
			config := &messaging.WebpushConfig{}
			addIconToConfig(tt.key, config)
			tt.validateConfig(t, config)
		})
	}
}

func TestAddToken(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name           string
		queryParams    map[string]string
		setupUserMap   func()
		expectedStatus int
		expectedBody   map[string]interface{}
		checkUserMap   func(*testing.T)
	}{
		{
			name: "add new token",
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
				"fcm_token":    "new_token",
			},
			setupUserMap: func() {
				userDeviceMap = make(map[string]map[string][]string)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "User Token added",
				},
			},
			checkUserMap: func(t *testing.T) {
				tokens := userDeviceMap["test_project_test_site"]["test_user"]
				assert.Equal(t, []string{"new_token"}, tokens)
			},
		},
		{
			name: "add duplicate token",
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
				"fcm_token":    "existing_token",
			},
			setupUserMap: func() {
				userDeviceMap = map[string]map[string][]string{
					"test_project_test_site": {
						"test_user": {"existing_token"},
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "User Token duplicate found",
				},
			},
			checkUserMap: func(t *testing.T) {
				tokens := userDeviceMap["test_project_test_site"]["test_user"]
				assert.Equal(t, []string{"existing_token"}, tokens)
			},
		},
		{
			name: "missing token",
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
			},
			setupUserMap: func() {
				userDeviceMap = make(map[string]map[string][]string)
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": false,
					"message": "FCM token is required",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			tt.setupUserMap()

			// Setup request with query parameters
			req, err := http.NewRequest(http.MethodPost, "/add-token", http.NoBody)
			require.NoError(t, err)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			addToken(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)

			if tt.checkUserMap != nil {
				tt.checkUserMap(t)
			}
		})
	}
}

func TestRemoveToken(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name           string
		queryParams    map[string]string
		setupUserMap   func()
		expectedStatus int
		expectedBody   map[string]interface{}
		checkUserMap   func(*testing.T)
	}{
		{
			name: "remove existing token",
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
				"fcm_token":    "existing_token",
			},
			setupUserMap: func() {
				userDeviceMap = map[string]map[string][]string{
					"test_project_test_site": {
						"test_user": {"existing_token", "other_token"},
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "User Token removed",
				},
			},
			checkUserMap: func(t *testing.T) {
				tokens := userDeviceMap["test_project_test_site"]["test_user"]
				assert.Equal(t, []string{"other_token"}, tokens)
			},
		},
		{
			name: "remove non-existent token",
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"user_id":      "test_user",
				"fcm_token":    "nonexistent_token",
			},
			setupUserMap: func() {
				userDeviceMap = map[string]map[string][]string{
					"test_project_test_site": {
						"test_user": {"existing_token"},
					},
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": true,
					"message": "User Token not found, removed",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			tt.setupUserMap()

			// Setup request with query parameters
			req, err := http.NewRequest(http.MethodPost, "/remove-token", http.NoBody)
			require.NoError(t, err)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			removeToken(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)

			if tt.checkUserMap != nil {
				tt.checkUserMap(t)
			}
		})
	}
}

func TestApplyTopicDecorations(t *testing.T) {
	tests := []struct {
		name             string
		topic            string
		title            string
		setupDecorations func()
		expected         string
	}{
		{
			name:  "no decorations",
			topic: "test_topic",
			title: "Test Title",
			setupDecorations: func() {
				topicDecorations = make(map[string]TopicDecoration)
			},
			expected: "Test Title",
		},
		{
			name:  "matching decoration",
			topic: "test_topic",
			title: "Alert: Test Message",
			setupDecorations: func() {
				topicDecorations = map[string]TopicDecoration{
					"test_topic": {
						Pattern:  "^Alert:",
						Template: "📢 {title}",
					},
				}
			},
			expected: "📢 Alert: Test Message",
		},
		{
			name:  "non-matching decoration",
			topic: "test_topic",
			title: "Normal Message",
			setupDecorations: func() {
				topicDecorations = map[string]TopicDecoration{
					"test_topic": {
						Pattern:  "^Alert:",
						Template: "📢 {title}",
					},
				}
			},
			expected: "Normal Message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupDecorations()
			result := applyTopicDecorations(tt.topic, tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSendNotificationToTopic(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	mockClient := &mocks.MockFirebaseMessagingClient{}
	messagingClient = mockClient

	tests := []struct {
		name           string
		setupMock      func()
		queryParams    map[string]string
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "successful notification",
			setupMock: func() {
				mockClient.On("Send",
					mock.Anything,
					mock.MatchedBy(func(msg *messaging.Message) bool {
						return msg.Topic == "test_topic" &&
							msg.Notification != nil &&
							msg.Notification.Title == "Test Title" &&
							msg.Notification.Body == "Test Body" &&
							msg.Webpush.Notification.Title == "Test Title" &&
							msg.Webpush.Notification.Body == "Test Body"
					}),
				).Return("message_id", nil)
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"topic_name":   "test_topic",
				"title":        "Test Title",
				"body":         "Test Body",
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"message": map[string]interface{}{
					"success": float64(200),
					"message": "Notification sent to test_topic topic",
				},
			},
		},
		{
			name:      "missing topic name",
			setupMock: func() {},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"title":        "Test Title",
				"body":         "Test Body",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"exc": map[string]interface{}{
					"status_code": float64(400),
					"message":     "topic_name is required",
				},
			},
		},
		{
			name:      "missing title",
			setupMock: func() {},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"topic_name":   "test_topic",
				"body":         "Test Body",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"exc": map[string]interface{}{
					"status_code": float64(400),
					"message":     "title is required",
				},
			},
		},
		{
			name: "firebase client error",
			setupMock: func() {
				mockClient.On("Send",
					mock.Anything,
					mock.Anything,
				).Return("", fmt.Errorf("firebase error"))
			},
			queryParams: map[string]string{
				"project_name": "test_project",
				"site_name":    "test_site",
				"topic_name":   "test_topic",
				"title":        "Test Title",
				"body":         "Test Body",
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"exc": map[string]interface{}{
					"status_code": float64(500),
					"message":     "Failed to send notification: firebase error",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := createTestContext(w)

			// Setup mock
			mockClient.ExpectedCalls = nil
			tt.setupMock()

			// Setup request with query parameters
			req, err := http.NewRequest(http.MethodPost, "/send-topic", http.NoBody)
			require.NoError(t, err)
			q := req.URL.Query()
			for k, v := range tt.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			sendNotificationToTopic(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)

			mockClient.AssertExpectations(t)
		})
	}
}

func TestGetUserTokens(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name        string
		key         string
		userID      string
		setupMap    func()
		expectError bool
		expected    []string
	}{
		{
			name:   "existing user with tokens",
			key:    "test_project_site",
			userID: "test_user",
			setupMap: func() {
				userDeviceMap = map[string]map[string][]string{
					"test_project_site": {
						"test_user": {"token1", "token2"},
					},
				}
			},
			expected: []string{"token1", "token2"},
		},
		{
			name:   "user with no tokens",
			key:    "test_project_site",
			userID: "test_user",
			setupMap: func() {
				userDeviceMap = map[string]map[string][]string{
					"test_project_site": {
						"test_user": {},
					},
				}
			},
			expectError: true,
		},
		{
			name:   "nonexistent user",
			key:    "test_project_site",
			userID: "nonexistent_user",
			setupMap: func() {
				userDeviceMap = map[string]map[string][]string{
					"test_project_site": {},
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMap()

			tokens, err := getUserTokens(tt.key, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not subscribed to push notifications")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, tokens)
			}
		})
	}
}

func TestAPIBasicAuth(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name             string
		setupAuth        func()
		setupHeader      func(*http.Request)
		expectedStatus   int
		expectAuthHeader bool
	}{
		{
			name: "valid credentials",
			setupAuth: func() {
				credentials["valid-key"] = "valid-secret"
			},
			setupHeader: func(req *http.Request) {
				req.SetBasicAuth("valid-key", "valid-secret")
			},
			expectedStatus:   http.StatusOK,
			expectAuthHeader: false,
		},
		{
			name:      "missing auth header",
			setupAuth: func() {},
			setupHeader: func(req *http.Request) {
				// No auth header
			},
			expectedStatus:   http.StatusUnauthorized,
			expectAuthHeader: true,
		},
		{
			name: "invalid credentials",
			setupAuth: func() {
				credentials["valid-key"] = "valid-secret"
			},
			setupHeader: func(req *http.Request) {
				req.SetBasicAuth("invalid-key", "invalid-secret")
			},
			expectedStatus:   http.StatusUnauthorized,
			expectAuthHeader: true,
		},
		{
			name: "wrong secret for valid key",
			setupAuth: func() {
				credentials["valid-key"] = "valid-secret"
			},
			setupHeader: func(req *http.Request) {
				req.SetBasicAuth("valid-key", "wrong-secret")
			},
			expectedStatus:   http.StatusUnauthorized,
			expectAuthHeader: true,
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

			// Create request
			req, err := http.NewRequest(http.MethodGet, "/test", http.NoBody)
			require.NoError(t, err)

			// Setup auth header
			tt.setupHeader(req)

			// Serve the request
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectAuthHeader {
				assert.Equal(t, "Basic realm=Authorization Required", w.Header().Get("WWW-Authenticate"))
			} else {
				assert.Empty(t, w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestGenerateSecureToken(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "32 characters",
			length: 32,
		},
		{
			name:   "48 characters",
			length: 48,
		},
		{
			name:   "64 characters",
			length: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := generateSecureToken(tt.length)
			assert.Len(t, token, tt.length)

			// Verify token contains only allowed characters
			for _, char := range token {
				assert.Contains(t, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", string(char))
			}

			// Generate another token and verify it's different
			anotherToken := generateSecureToken(tt.length)
			assert.NotEqual(t, token, anotherToken, "Tokens should be random")
		})
	}
}
