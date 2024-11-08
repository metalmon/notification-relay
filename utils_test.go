package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	// Save original configPath
	originalConfigPath := configPath
	defer func() {
		configPath = originalConfigPath
	}()

	configPath = "/tmp/test/config.json"

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "Valid config file",
			filename: ConfigJSON,
			wantErr:  false,
		},
		{
			name:     "Valid credentials file",
			filename: CredentialsJSON,
			wantErr:  false,
		},
		{
			name:     "Unauthorized file",
			filename: "unauthorized.json",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tt.wantErr {
					t.Errorf("getConfigPath() panic = %v, wantErr %v", r, tt.wantErr)
				}
			}()

			got := getConfigPath(tt.filename)
			expected := filepath.Join(filepath.Dir(configPath), tt.filename)
			if !tt.wantErr && got != expected {
				t.Errorf("getConfigPath() = %v, want %v", got, expected)
			}
		})
	}
}

func TestLoadJSON(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Save original configPath
	originalConfigPath := configPath
	defer func() {
		configPath = originalConfigPath
	}()

	configPath = filepath.Join(tmpDir, ConfigJSON)

	// Create test data
	testData := map[string]string{"test": "value"}
	testFile := filepath.Join(tmpDir, CredentialsJSON)
	data, _ := json.Marshal(testData)
	if err := os.WriteFile(testFile, data, 0o600); err != nil {
		t.Fatal(err)
	}

	var result map[string]string
	if err := loadJSON(CredentialsJSON, &result); err != nil {
		t.Errorf("loadJSON() error = %v", err)
	}

	if result["test"] != "value" {
		t.Errorf("loadJSON() got = %v, want %v", result["test"], "value")
	}
}

func TestSaveJSON(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Save original configPath
	originalConfigPath := configPath
	defer func() {
		configPath = originalConfigPath
	}()

	configPath = filepath.Join(tmpDir, ConfigJSON)

	testData := map[string]string{"test": "value"}

	if err := saveJSON(CredentialsJSON, testData); err != nil {
		t.Errorf("saveJSON() error = %v", err)
	}

	// Verify the file was saved correctly
	var result map[string]string
	if err := loadJSON(CredentialsJSON, &result); err != nil {
		t.Errorf("Failed to verify saved JSON: %v", err)
	}

	if result["test"] != "value" {
		t.Errorf("saveJSON() got = %v, want %v", result["test"], "value")
	}
}
