package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyDecorations(t *testing.T) {
	// Инициализируем тестовые декорации
	decorations = make(map[string]map[string]Decoration)
	testKey := "test-project_test-site"
	decorations[testKey] = map[string]Decoration{
		"alert": {
			Pattern:  "^Alert:",
			Template: "🚨 {title}",
		},
		"info": {
			Pattern:  "^Info:",
			Template: "ℹ️ {title}",
		},
	}

	tests := []struct {
		name     string
		key      string
		title    string
		expected string
	}{
		{
			name:     "Apply alert decoration",
			key:      testKey,
			title:    "Alert: System error",
			expected: "🚨 Alert: System error",
		},
		{
			name:     "Apply info decoration",
			key:      testKey,
			title:    "Info: Update available",
			expected: "ℹ️ Info: Update available",
		},
		{
			name:     "No matching decoration",
			key:      testKey,
			title:    "Regular message",
			expected: "Regular message",
		},
		{
			name:     "Non-existent project",
			key:      "non-existent",
			title:    "Alert: Test",
			expected: "Alert: Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyDecorations(tt.key, tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrepareWebPushConfig(t *testing.T) {
	// Инициализируем тестовые декорации и иконки
	decorations = make(map[string]map[string]Decoration)
	icons = make(map[string]string)
	testKey := "test-project_test-site"

	decorations[testKey] = map[string]Decoration{
		"alert": {
			Pattern:  "^Alert:",
			Template: "🚨 {title}",
		},
	}
	icons[testKey] = "/path/to/icon.png"

	tests := []struct {
		name        string
		key         string
		title       string
		body        string
		data        string
		wantErr     bool
		checkIcon   bool
		checkAction bool
	}{
		{
			name:      "Basic notification",
			key:       testKey,
			title:     "Test Title",
			body:      "Test Body",
			data:      "",
			wantErr:   false,
			checkIcon: true,
		},
		{
			name:      "With decoration",
			key:       testKey,
			title:     "Alert: Test",
			body:      "Test Body",
			data:      "",
			wantErr:   false,
			checkIcon: true,
		},
		{
			name:        "With click action",
			key:         testKey,
			title:       "Test Title",
			body:        "Test Body",
			data:        `{"click_action": "https://example.com"}`,
			wantErr:     false,
			checkIcon:   true,
			checkAction: true,
		},
		{
			name:    "Invalid data JSON",
			key:     testKey,
			title:   "Test Title",
			body:    "Test Body",
			data:    `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := prepareWebPushConfig(tt.key, tt.title, tt.body, tt.data)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, config)
			assert.NotNil(t, config.Notification)
			assert.Equal(t, tt.body, config.Notification.Body)

			// Проверяем декорацию заголовка
			if strings.HasPrefix(tt.title, "Alert:") {
				assert.Equal(t, "🚨 "+tt.title, config.Notification.Title)
			} else {
				assert.Equal(t, tt.title, config.Notification.Title)
			}

			// Проверяем иконку
			if tt.checkIcon {
				assert.NotNil(t, config.Data)
				assert.Equal(t, "/path/to/icon.png", config.Data["icon"])
			}

			// Проверяем click_action
			if tt.checkAction {
				assert.NotNil(t, config.FCMOptions)
				assert.Equal(t, "https://example.com", config.FCMOptions.Link)
			}
		})
	}
}

func TestApplyTopicDecorations(t *testing.T) {
	// Инициализируем тестовые декорации для топиков
	topicDecorations = make(map[string]TopicDecoration)
	topicDecorations["news"] = TopicDecoration{
		Pattern:  "^News:",
		Template: "📰 {title}",
	}
	topicDecorations["alerts"] = TopicDecoration{
		Pattern:  "^Alert:",
		Template: "🚨 {title}",
	}

	tests := []struct {
		name     string
		topic    string
		title    string
		expected string
	}{
		{
			name:     "Apply news decoration",
			topic:    "news",
			title:    "News: Latest update",
			expected: "📰 News: Latest update",
		},
		{
			name:     "Apply alert decoration",
			topic:    "alerts",
			title:    "Alert: System error",
			expected: "🚨 Alert: System error",
		},
		{
			name:     "No matching decoration",
			topic:    "news",
			title:    "Regular message",
			expected: "Regular message",
		},
		{
			name:     "Non-existent topic",
			topic:    "non-existent",
			title:    "News: Test",
			expected: "News: Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyTopicDecorations(tt.topic, tt.title)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrepareTopicWebPushConfig(t *testing.T) {
	// Инициализируем тестовые декорации для топиков
	topicDecorations = make(map[string]TopicDecoration)
	topicDecorations["news"] = TopicDecoration{
		Pattern:  "^News:",
		Template: "📰 {title}",
	}

	tests := []struct {
		name        string
		topic       string
		title       string
		body        string
		data        string
		wantErr     bool
		checkIcon   bool
		checkAction bool
	}{
		{
			name:    "Basic topic notification",
			topic:   "news",
			title:   "News: Latest update",
			body:    "Test Body",
			data:    "",
			wantErr: false,
		},
		{
			name:        "With click action",
			topic:       "news",
			title:       "News: Test",
			body:        "Test Body",
			data:        `{"click_action": "https://example.com"}`,
			wantErr:     false,
			checkAction: true,
		},
		{
			name:      "With custom icon",
			topic:     "news",
			title:     "News: Test",
			body:      "Test Body",
			data:      `{"icon": "/custom/icon.png"}`,
			wantErr:   false,
			checkIcon: true,
		},
		{
			name:    "Invalid data JSON",
			topic:   "news",
			title:   "News: Test",
			body:    "Test Body",
			data:    `{invalid json}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := prepareTopicWebPushConfig(tt.topic, tt.title, tt.body, tt.data)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, config)
			assert.NotNil(t, config.Notification)
			assert.Equal(t, tt.body, config.Notification.Body)

			// Проверяем декорацию заголовка для новостей
			if strings.HasPrefix(tt.title, "News:") {
				assert.Equal(t, "📰 "+tt.title, config.Notification.Title)
			} else {
				assert.Equal(t, tt.title, config.Notification.Title)
			}

			// Проверяем иконку
			if tt.checkIcon {
				assert.NotNil(t, config.Data)
				assert.Equal(t, "/custom/icon.png", config.Data["icon"])
			}

			// Проверяем click_action
			if tt.checkAction {
				assert.NotNil(t, config.FCMOptions)
				assert.Equal(t, "https://example.com", config.FCMOptions.Link)
			}
		})
	}
}
