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

	// Регулярное выражение для проверки допустимых символов
	validChars := regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Генерируем токен
			token := generateSecureToken(tt.length)

			// Проверяем длину
			if len(token) != tt.length {
				t.Errorf("generateSecureToken() length = %v, want %v", len(token), tt.length)
			}

			// Проверяем, что токен содержит только допустимые символы
			if !validChars.MatchString(token) {
				t.Errorf("generateSecureToken() contains invalid characters: %v", token)
			}

			// Проверяем уникальность - генерируем второй токен и сравниваем
			token2 := generateSecureToken(tt.length)
			if token == token2 {
				t.Error("generateSecureToken() generated identical tokens")
			}
		})
	}
}

func TestGetCredential(t *testing.T) {
	// Переключаем Gin в тестовый режим
	gin.SetMode(gin.TestMode)

	// Инициализируем credentials map
	credentials = make(Credentials)

	// Создаем тестовый HTTP сервер для эмуляции webhook
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/valid-webhook" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("valid-token"))
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
				Endpoint:     testServer.URL[7:], // удаляем "http://"
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
			// Создаем новый тестовый контекст Gin для каждого теста
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Подготавливаем JSON запрос
			jsonData, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			// Создаем новый запрос с JSON данными
			c.Request, err = http.NewRequest(
				http.MethodPost,
				"/api/method/notification_relay.api.auth.get_credential",
				bytes.NewBuffer(jsonData),
			)
			assert.NoError(t, err)
			c.Request.Header.Set("Content-Type", "application/json")

			// Вызываем тестируемую функцию
			getCredential(c)

			// Проверяем статус ответа
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Разбираем ответ
			var response CredentialResponse
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			// Проверяем успешность операции
			assert.Equal(t, tt.wantSuccess, response.Success)

			// Если ожидается сообщение об ошибке, проверяем его
			if tt.wantMessage != "" {
				assert.Equal(t, tt.wantMessage, response.Message)
			}

			// Для успешного запроса проверяем наличие учетных данных
			if tt.wantSuccess {
				assert.NotNil(t, response.Credentials)
				assert.NotEmpty(t, response.Credentials.APIKey)
				assert.NotEmpty(t, response.Credentials.APISecret)
			}
		})
	}
}

func TestAPIBasicAuth(t *testing.T) {
	// Переключаем Gin в тестовый режим
	gin.SetMode(gin.TestMode)

	// Инициализируем тестовые учетные данные
	credentials = make(Credentials)
	credentials["test-key"] = "test-secret"

	tests := []struct {
		name            string
		apiKey          string
		apiSecret       string
		expectedStatus  int
		checkAuthHeader bool // Добавляем флаг для проверки заголовка
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
			checkAuthHeader: true, // Проверяем заголовок только для отсутствующих credentials
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем тестовый роутер
			router := gin.New()

			// Добавляем middleware и тестовый обработчик
			router.Use(apiBasicAuth())
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Создаем тестовый запрос
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)

			// Добавляем Basic Auth заголовок, если есть учетные данные
			if tt.apiKey != "" || tt.apiSecret != "" {
				req.SetBasicAuth(tt.apiKey, tt.apiSecret)
			}

			// Выполняем запрос
			router.ServeHTTP(w, req)

			// Проверяем статус ответа
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Проверяем заголовк WWW-Authenticate тол��ко если это необходимо
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
	// Инициализируем тестовые иконки
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
				// Для проекта без иконки проверяем, что иконка не добавлена
				if tt.config.Data != nil {
					assert.Empty(t, tt.config.Data["icon"])
				}
			} else {
				// Для проекта с иконкой проверяем, что она добавлена корректно
				assert.NotNil(t, tt.config.Data)
				assert.Equal(t, tt.want, tt.config.Data["icon"])
			}
		})
	}
}
