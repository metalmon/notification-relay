package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Key tests needed:

// 1. Test addToken handler
func TestAddToken(t *testing.T) {
	// Переключаем Gin в тестовый режим
	gin.SetMode(gin.TestMode)

	// Инициализируем тестовую карту устройств
	userDeviceMap = make(map[string]map[string][]string)

	tests := []struct {
		name           string
		projectName    string
		siteName       string
		userID         string
		fcmToken       string
		expectedStatus int
		wantSuccess    bool
		wantMessage    string
	}{
		{
			name:           "Add new token for new user",
			projectName:    "test-project",
			siteName:       "test-site",
			userID:         "user1",
			fcmToken:       "token1",
			expectedStatus: http.StatusOK,
			wantSuccess:    true,
			wantMessage:    "User Token added",
		},
		{
			name:           "Add duplicate token",
			projectName:    "test-project",
			siteName:       "test-site",
			userID:         "user1",
			fcmToken:       "token1",
			expectedStatus: http.StatusOK,
			wantSuccess:    true,
			wantMessage:    "User Token duplicate found",
		},
		{
			name:           "Add token without FCM token",
			projectName:    "test-project",
			siteName:       "test-site",
			userID:         "user1",
			fcmToken:       "",
			expectedStatus: http.StatusOK,
			wantSuccess:    false,
			wantMessage:    "FCM token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем новый тестовый контекст Gin для каждого теста
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Устанавливаем query параметры
			c.Request, _ = http.NewRequest(http.MethodPost, "/test", nil)
			q := c.Request.URL.Query()
			q.Add("project_name", tt.projectName)
			q.Add("site_name", tt.siteName)
			q.Add("user_id", tt.userID)
			q.Add("fcm_token", tt.fcmToken)
			c.Request.URL.RawQuery = q.Encode()

			// Вызываем тестируемую функцию
			addToken(c)

			// Проверяем статус ответа
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Разбираем ответ
			var response Response
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Проверяем сообщение в ответе
			if tt.wantMessage != "" {
				message := response.Message.(map[string]interface{})
				assert.Equal(t, tt.wantSuccess, message["success"])
				assert.Equal(t, tt.wantMessage, message["message"])
			}

			// Для успешного добавления проверяем, что токен действительно добавлен
			if tt.wantSuccess && tt.wantMessage == "User Token added" {
				key := tt.projectName + "_" + tt.siteName
				tokens, exists := userDeviceMap[key][tt.userID]
				assert.True(t, exists)
				assert.Contains(t, tokens, tt.fcmToken)
			}
		})
	}
}

// 2. Test removeToken handler
func TestRemoveToken(t *testing.T) {
	// Переключаем Gin в тестовый режим
	gin.SetMode(gin.TestMode)

	// Инициализируем тестовую карту устройств с некоторыми данными
	userDeviceMap = make(map[string]map[string][]string)
	testKey := "test-project_test-site"
	testUserID := "user1"
	testToken := "token1"

	userDeviceMap[testKey] = make(map[string][]string)
	userDeviceMap[testKey][testUserID] = []string{testToken}

	tests := []struct {
		name           string
		projectName    string
		siteName       string
		userID         string
		fcmToken       string
		expectedStatus int
		wantSuccess    bool
		wantMessage    string
	}{
		{
			name:           "Remove existing token",
			projectName:    "test-project",
			siteName:       "test-site",
			userID:         "user1",
			fcmToken:       "token1",
			expectedStatus: http.StatusOK,
			wantSuccess:    true,
			wantMessage:    "User Token removed",
		},
		{
			name:           "Remove non-existent token",
			projectName:    "test-project",
			siteName:       "test-site",
			userID:         "user1",
			fcmToken:       "non-existent-token",
			expectedStatus: http.StatusOK,
			wantSuccess:    true,
			wantMessage:    "User Token not found, removed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем новый тестовый контекст Gin для каждого теста
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Устанавливаем query параметры
			c.Request, _ = http.NewRequest(http.MethodPost, "/test", nil)
			q := c.Request.URL.Query()
			q.Add("project_name", tt.projectName)
			q.Add("site_name", tt.siteName)
			q.Add("user_id", tt.userID)
			q.Add("fcm_token", tt.fcmToken)
			c.Request.URL.RawQuery = q.Encode()

			// Вызываем тестируемую функцию
			removeToken(c)

			// Проверяем статус ответа
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Разбираем ответ
			var response Response
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Проверяем сообщение в ответе
			message := response.Message.(map[string]interface{})
			assert.Equal(t, tt.wantSuccess, message["success"])
			assert.Equal(t, tt.wantMessage, message["message"])

			// Для успешного удаления проверяем, что токен действительно удален
			if tt.wantSuccess && tt.wantMessage == "User Token removed" {
				key := tt.projectName + "_" + tt.siteName
				tokens, exists := userDeviceMap[key][tt.userID]
				assert.True(t, exists)
				assert.NotContains(t, tokens, tt.fcmToken)
			}
		})
	}
}

// 3. Test getUserTokens
func TestGetUserTokens(t *testing.T) {
	// Инициализируем тестовую карту устройств
	userDeviceMap = make(map[string]map[string][]string)
	testKey := "test-project_test-site"
	testUserID := "user1"
	testTokens := []string{"token1", "token2"}

	// Подготавливаем тестовые данные
	userDeviceMap[testKey] = make(map[string][]string)
	userDeviceMap[testKey][testUserID] = testTokens

	tests := []struct {
		name       string
		key        string
		userID     string
		wantErr    bool
		wantTokens []string
	}{
		{
			name:       "Get tokens for existing user",
			key:        testKey,
			userID:     testUserID,
			wantErr:    false,
			wantTokens: testTokens,
		},
		{
			name:       "Get tokens for non-existent user",
			key:        testKey,
			userID:     "non-existent-user",
			wantErr:    true,
			wantTokens: nil,
		},
		{
			name:       "Get tokens for non-existent project",
			key:        "non-existent-project_site",
			userID:     testUserID,
			wantErr:    true,
			wantTokens: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Вызываем тестируемую функцию
			tokens, err := getUserTokens(tt.key, tt.userID)

			// Проверяем наличие ошибки
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, tokens)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTokens, tokens)
			}
		})
	}
}
