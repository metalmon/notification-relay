package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigPath(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name          string
		filename      string
		expectedPanic bool
	}{
		{
			name:          "valid config file",
			filename:      ConfigJSON,
			expectedPanic: false,
		},
		{
			name:          "unauthorized file",
			filename:      "unauthorized.json",
			expectedPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedPanic {
				assert.Panics(t, func() {
					getConfigPath(tt.filename)
				})
				return
			}

			path := getConfigPath(tt.filename)
			assert.Equal(t, filepath.Join(filepath.Dir(configPath), tt.filename), path)
		})
	}
}

func TestLoadJSON(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name        string
		setupFile   func(string) error
		filename    string
		expectError bool
	}{
		{
			name: "valid json file",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte(`{"test": "value"}`), 0644)
			},
			filename:    "test.json",
			expectError: false,
		},
		{
			name: "invalid json content",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte(`invalid json`), 0644)
			},
			filename:    "test.json",
			expectError: true,
		},
		{
			name: "non-json extension",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte(`{"test": "value"}`), 0644)
			},
			filename:    "test.txt",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowedFiles[tt.filename] = true
			defer delete(allowedFiles, tt.filename)

			path := filepath.Join(tmpDir, tt.filename)
			err := tt.setupFile(path)
			require.NoError(t, err)

			var result map[string]interface{}
			err = loadJSON(tt.filename, &result)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "value", result["test"])
			}
		})
	}
}

func TestEnsureFileExists(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name         string
		filename     string
		defaultValue interface{}
		checkResult  func(*testing.T, string)
	}{
		{
			name:     "create new file",
			filename: "test.json",
			defaultValue: map[string]string{
				"key": "value",
			},
			checkResult: func(t *testing.T, path string) {
				assert.FileExists(t, path)

				var result map[string]string
				err := loadJSON("test.json", &result)
				require.NoError(t, err)
				assert.Equal(t, "value", result["key"])
			},
		},
		{
			name:     "existing file unchanged",
			filename: "existing.json",
			defaultValue: map[string]string{
				"new": "value",
			},
			checkResult: func(t *testing.T, path string) {
				var result map[string]string
				err := loadJSON("existing.json", &result)
				require.NoError(t, err)
				assert.Equal(t, "original", result["existing"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowedFiles[tt.filename] = true
			defer delete(allowedFiles, tt.filename)

			if tt.name == "existing file unchanged" {
				// Create existing file
				existingData := map[string]string{"existing": "original"}
				path := filepath.Join(tmpDir, tt.filename)
				writeTestJSON(t, path, existingData)
			}

			ensureFileExists(tt.filename, tt.defaultValue)

			path := getConfigPath(tt.filename)
			tt.checkResult(t, path)
		})
	}
}

func TestSaveJSON(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name        string
		filename    string
		data        interface{}
		setup       func()
		expectError bool
		errorMsg    string
	}{
		{
			name:     "save valid data",
			filename: "test.json",
			data: map[string]string{
				"key": "value",
			},
			setup: func() {
				allowedFiles["test.json"] = true
			},
			expectError: false,
		},
		{
			name:     "save to unauthorized file",
			filename: "unauthorized.json",
			data: map[string]string{
				"key": "value",
			},
			setup:       func() {},
			expectError: true,
			errorMsg:    "Unauthorized file access attempt: unauthorized.json",
		},
		{
			name:     "invalid json data",
			filename: "test.json",
			data: map[string]interface{}{
				"invalid": make(chan int), // channels cannot be marshaled to JSON
			},
			setup: func() {
				allowedFiles["test.json"] = true
			},
			expectError: true,
			errorMsg:    "Failed to marshal JSON: json: unsupported type: chan int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()

			if tt.expectError && tt.errorMsg == "Unauthorized file access attempt: unauthorized.json" {
				assert.PanicsWithValue(t, tt.errorMsg, func() {
					_ = saveJSON(tt.filename, tt.data)
				})
				return
			}

			if tt.expectError && strings.Contains(tt.errorMsg, "Failed to marshal JSON") {
				assert.PanicsWithValue(t, tt.errorMsg, func() {
					_ = saveJSON(tt.filename, tt.data)
				})
				return
			}

			err := saveJSON(tt.filename, tt.data)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify file contents
				var result map[string]string
				err = loadJSON(tt.filename, &result)
				require.NoError(t, err)
				assert.Equal(t, tt.data, result)
			}
		})
	}
}
