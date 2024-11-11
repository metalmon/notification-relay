package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitFirebase(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create a test service account file
	serviceAccountContent := `{
		"type": "service_account",
		"project_id": "test-project",
		"private_key": "test-key",
		"client_email": "test@test.com"
	}`
	serviceAccountPath = filepath.Join(tmpDir, "service-account.json")
	err := os.WriteFile(serviceAccountPath, []byte(serviceAccountContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		setupEnv    func()
		expectError bool
	}{
		{
			name: "valid service account",
			setupEnv: func() {
				os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", serviceAccountPath)
			},
			expectError: false,
		},
		{
			name: "missing service account",
			setupEnv: func() {
				os.Remove(serviceAccountPath)
				os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			err := initFirebase()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetTrustedProxies(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies string
		expectError    bool
		validateEngine func(*testing.T, *gin.Engine)
	}{
		{
			name:           "trust all proxies",
			trustedProxies: "*",
			expectError:    false,
			validateEngine: func(t *testing.T, engine *gin.Engine) {
				assert.NotNil(t, engine)
			},
		},
		{
			name:           "trust no proxies",
			trustedProxies: "none",
			expectError:    false,
			validateEngine: func(t *testing.T, engine *gin.Engine) {
				assert.NotNil(t, engine)
			},
		},
		{
			name:           "valid CIDR",
			trustedProxies: "127.0.0.1/32,10.0.0.0/8",
			expectError:    false,
			validateEngine: func(t *testing.T, engine *gin.Engine) {
				assert.NotNil(t, engine)
			},
		},
		{
			name:           "invalid CIDR",
			trustedProxies: "invalid-cidr",
			expectError:    true,
		},
		{
			name:           "empty string",
			trustedProxies: "",
			expectError:    false,
			validateEngine: func(t *testing.T, engine *gin.Engine) {
				assert.NotNil(t, engine)
			},
		},
		{
			name:           "whitespace string",
			trustedProxies: "  ",
			expectError:    false,
			validateEngine: func(t *testing.T, engine *gin.Engine) {
				assert.NotNil(t, engine)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			err := setTrustedProxies(engine, tt.trustedProxies)

			if tt.expectError {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), "invalid CIDR")
				}
			} else {
				assert.NoError(t, err)
				if tt.validateEngine != nil {
					tt.validateEngine(t, engine)
				}
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		expectError bool
	}{
		{
			name:        "valid IPv4 CIDR",
			cidr:        "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "valid IPv6 CIDR",
			cidr:        "2001:db8::/32",
			expectError: false,
		},
		{
			name:        "invalid CIDR format",
			cidr:        "invalid",
			expectError: true,
		},
		{
			name:        "invalid network bits",
			cidr:        "192.168.1.0/33",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := net.ParseCIDR(tt.cidr)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
