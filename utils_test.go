package main

import (
	"os"
	"path/filepath"
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