package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigFileOperations(t *testing.T) {
	// Create test directory
	testDir := filepath.Join(filepath.Dir(configPath), "test_config_ops")
	err := os.MkdirAll(testDir, 0700)
	assert.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Save original config path
	originalConfigPath := configPath
	defer func() { configPath = originalConfigPath }()

	// Test loading non-existent file
	nonExistentPath := filepath.Join(testDir, ConfigJSON)
	configPath = nonExistentPath
	var cfg Config
	err = loadJSON(ConfigJSON, &cfg)
	assert.Error(t, err, "Should error when loading non-existent file")

	// Test loading file with invalid permissions
	readOnlyDir := filepath.Join(testDir, "readonly")
	err = os.MkdirAll(readOnlyDir, 0500)
	assert.NoError(t, err)
	configPath = filepath.Join(readOnlyDir, ConfigJSON)
	err = saveJSON(ConfigJSON, Config{})
	assert.Error(t, err, "Should error when saving to read-only directory")

	// Test saving and loading valid data
	configPath = filepath.Join(testDir, ConfigJSON)
	testData := Config{
		VapidPublicKey: "test_key",
		FirebaseConfig: map[string]interface{}{
			"project_id": "test-project",
		},
	}

	// Test saving data
	err = saveJSON(ConfigJSON, testData)
	assert.NoError(t, err)

	// Test loading saved data
	var loadedData Config
	err = loadJSON(ConfigJSON, &loadedData)
	assert.NoError(t, err)
	assert.Equal(t, testData.VapidPublicKey, loadedData.VapidPublicKey)
	assert.Equal(t, testData.FirebaseConfig["project_id"], loadedData.FirebaseConfig["project_id"])

	// Test loading with wrong type
	var wrongType struct {
		InvalidField int `json:"vapid_public_key"`
	}
	err = loadJSON(ConfigJSON, &wrongType)
	assert.Error(t, err, "Should error when loading into incompatible type")

	// Test saving invalid JSON
	invalidData := make(chan int)
	assert.Panics(t, func() {
		saveJSON(ConfigJSON, invalidData)
	}, "Should panic when trying to marshal invalid data")

	// Test file path security
	assert.Panics(t, func() {
		loadJSON("../config.json", &cfg)
	}, "Should panic on path traversal attempt")

	assert.Panics(t, func() {
		loadJSON("/etc/passwd", &cfg)
	}, "Should panic on absolute path")

	assert.Panics(t, func() {
		loadJSON("unauthorized.json", &cfg)
	}, "Should panic on unauthorized file")
}

func TestConfigFilePermissions(t *testing.T) {
	// Create test directory
	testDir := filepath.Join(filepath.Dir(configPath), "test_permissions")
	err := os.MkdirAll(testDir, 0700)
	assert.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Save original config path
	originalConfigPath := configPath
	defer func() { configPath = originalConfigPath }()

	// Test directory with no write permissions
	readOnlyDir := filepath.Join(testDir, "readonly")
	err = os.MkdirAll(readOnlyDir, 0500)
	assert.NoError(t, err)

	configPath = filepath.Join(readOnlyDir, ConfigJSON)
	err = saveJSON(ConfigJSON, Config{})
	assert.Error(t, err, "Should error when saving to read-only directory")

	// Test directory with no read permissions
	writeOnlyDir := filepath.Join(testDir, "writeonly")
	err = os.MkdirAll(writeOnlyDir, 0300)
	assert.NoError(t, err)

	configPath = filepath.Join(writeOnlyDir, ConfigJSON)
	var cfg Config
	err = loadJSON(ConfigJSON, &cfg)
	assert.Error(t, err, "Should error when reading from write-only directory")
}
