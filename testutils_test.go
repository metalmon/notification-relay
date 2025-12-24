package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	defaultFileMode = 0o644
	defaultDirMode  = 0o755
)

// setupTestEnvironment sets up a test environment with temporary config files
func setupTestEnvironment(t *testing.T) (tmpDir string, cleanup func()) {
	// Create temp directory for test configs
	tmpDir, err := os.MkdirTemp("", "notification-relay-test-*")
	require.NoError(t, err)

	// Assign configPath to a test config file
	configPath = filepath.Join(tmpDir, ConfigJSON)

	// Create Projects in config
	config = Config{
		Projects: map[string]ProjectConfig{
			"test_project": {
				VapidPublicKey: "test-vapid-key",
				FirebaseConfig: FirebaseConfig{
					ApiKey: "test-api-key",
				},
			},
		},
		TrustedProxies: "127.0.0.1",
	}

	writeTestJSON(t, configPath, config)
	writeTestJSON(t, filepath.Join(tmpDir, CredentialsJSON), make(Credentials))
	writeTestJSON(t, filepath.Join(tmpDir, UserDeviceMapJSON), make(map[string]map[string][]string))
	writeTestJSON(t, filepath.Join(tmpDir, DecorationJSON), make(map[string]map[string]Decoration))
	writeTestJSON(t, filepath.Join(tmpDir, TopicDecorationJSON), make(map[string]TopicDecoration))
	writeTestJSON(t, filepath.Join(tmpDir, IconsJSON), make(map[string]string))

	// Reset global variables
	credentials = make(Credentials)
	userDeviceMap = make(map[string]map[string][]string)
	decorations = make(map[string]map[string]Decoration)
	topicDecorations = make(map[string]TopicDecoration)
	icons = make(map[string]string)

	// Initialize test environment
	gin.SetMode(gin.TestMode)

	cleanup = func() {
		configPath = ""
		serviceAccountPath = ""
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Errorf("Failed to cleanup test directory: %v", err)
		}
	}

	return tmpDir, cleanup
}

// writeTestJSON writes test data to a JSON file
func writeTestJSON(t *testing.T, path string, data interface{}) {
	err := os.MkdirAll(filepath.Dir(path), defaultDirMode)
	require.NoError(t, err)

	file, err := json.MarshalIndent(data, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(path, file, defaultFileMode)
	require.NoError(t, err)
}

// createTestContext creates a new Gin context for testing
func createTestContext(w *httptest.ResponseRecorder) (*gin.Context, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	c, _ := gin.CreateTestContext(w)
	return c, router
}

// makeTestRequest creates and returns a test request with the given parameters
func makeTestRequest(method, path string, body interface{}) (*http.Request, error) {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, path, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
