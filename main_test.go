package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var originalInitFirebase = initFirebase // Store the original function

func TestInitFirebase(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	defer func() {
		initFirebase = originalInitFirebase // Restore the original function after all tests
	}()

	originalServiceAccountPath := serviceAccountPath
	defer func() {
		serviceAccountPath = originalServiceAccountPath
	}()

	tests := []struct {
		name        string
		setupEnv    func()
		mockError   error // Add this field to store the expected error
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid service account but fails Firebase init",
			setupEnv: func() {
				serviceAccountPath = filepath.Join(tmpDir, "service-account.json")
				// #nosec G101 -- test credentials only
				content := `{
					"type": "service_account",
					"project_id": "test-project",
					"private_key_id": "test-key-id",
					"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQC9QFi67K5UHhw5\n-----END PRIVATE KEY-----\n",
					"client_email": "test@test-project.iam.gserviceaccount.com",
					"client_id": "test-client-id",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test"
				}`
				err := os.WriteFile(serviceAccountPath, []byte(content), defaultFileMode)
				require.NoError(t, err)
			},
			mockError:   fmt.Errorf("failed to initialize Firebase: oauth2/google: invalid JWT signature"),
			expectError: true,
			errorMsg:    "failed to initialize Firebase: oauth2/google: invalid JWT signature",
		},
		{
			name: "invalid service account json",
			setupEnv: func() {
				serviceAccountPath = filepath.Join(tmpDir, "service-account.json")
				err := os.WriteFile(serviceAccountPath, []byte("invalid json"), defaultFileMode)
				require.NoError(t, err)
			},
			mockError:   fmt.Errorf("failed to initialize Firebase: invalid service account JSON"),
			expectError: true,
			errorMsg:    "failed to initialize Firebase: invalid service account JSON",
		},
		{
			name: "missing service account file",
			setupEnv: func() {
				serviceAccountPath = filepath.Join(tmpDir, "nonexistent.json")
			},
			mockError:   fmt.Errorf("failed to initialize Firebase: service account file not found"),
			expectError: true,
			errorMsg:    "failed to initialize Firebase: service account file not found",
		},
		{
			name: "empty credentials path",
			setupEnv: func() {
				serviceAccountPath = ""
			},
			mockError:   fmt.Errorf("failed to initialize Firebase: no service account file found"),
			expectError: true,
			errorMsg:    "failed to initialize Firebase: no service account file found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset environment and global variables before each test
			serviceAccountPath = ""
			messagingClient = nil

			// Set up the mock function for this specific test
			initFirebase = func() error {
				return tt.mockError
			}

			tt.setupEnv()
			err := initFirebase()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, messagingClient)
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

func TestInitCredentials(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name          string
		setupFile     func()
		validateCreds func(*testing.T)
	}{
		{
			name: "load existing credentials",
			setupFile: func() {
				testCreds := map[string]string{
					"test-key": "test-secret",
				}
				writeTestJSON(t, filepath.Join(tmpDir, CredentialsJSON), testCreds)
			},
			validateCreds: func(t *testing.T) {
				assert.Equal(t, "test-secret", credentials["test-key"])
			},
		},
		{
			name: "create new credentials file",
			setupFile: func() {
				// Don't create the file - it should be created
				credPath := filepath.Join(tmpDir, CredentialsJSON)
				if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			},
			validateCreds: func(t *testing.T) {
				assert.Empty(t, credentials)
				assert.FileExists(t, filepath.Join(tmpDir, CredentialsJSON))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset credentials
			credentials = make(map[string]string)

			tt.setupFile()
			initCredentials()

			if tt.validateCreds != nil {
				tt.validateCreds(t)
			}
		})
	}
}

func TestLoadDataFiles(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name         string
		setupFiles   func()
		validateData func(*testing.T)
	}{
		{
			name: "load all data files",
			setupFiles: func() {
				// Add files to allowed files map first
				allowedFiles[UserDeviceMapJSON] = true
				allowedFiles[DecorationJSON] = true
				allowedFiles[TopicDecorationJSON] = true
				allowedFiles[IconsJSON] = true

				// Create test data files
				userDevices := map[string]map[string][]string{
					"test_project": {
						"test_user": {"token1"},
					},
				}
				writeTestJSON(t, filepath.Join(tmpDir, UserDeviceMapJSON), userDevices)

				decorationData := map[string]map[string]Decoration{
					"test_project": {
						"alert": {Pattern: "^Alert:", Template: "🚨 {title}"},
					},
				}
				writeTestJSON(t, filepath.Join(tmpDir, DecorationJSON), decorationData)

				topicDecorationData := map[string]TopicDecoration{
					"test_topic": {Pattern: "^Alert:", Template: "📢 {title}"},
				}
				writeTestJSON(t, filepath.Join(tmpDir, TopicDecorationJSON), topicDecorationData)

				iconData := map[string]string{
					"test_project": "/path/to/icon.png",
				}
				writeTestJSON(t, filepath.Join(tmpDir, IconsJSON), iconData)
			},
			validateData: func(t *testing.T) {
				// Verify user device map
				tokens, exists := userDeviceMap["test_project"]["test_user"]
				assert.True(t, exists, "user device map entry should exist")
				assert.Equal(t, []string{"token1"}, tokens)

				// Verify decorations
				projectDec, exists := decorations["test_project"]
				assert.True(t, exists, "project decorations should exist")
				dec, exists := projectDec["alert"]
				assert.True(t, exists, "alert decoration should exist")
				assert.Equal(t, "^Alert:", dec.Pattern)
				assert.Equal(t, "🚨 {title}", dec.Template)

				// Verify topic decorations
				topicDec, exists := topicDecorations["test_topic"]
				assert.True(t, exists, "topic decoration should exist")
				assert.Equal(t, "^Alert:", topicDec.Pattern)
				assert.Equal(t, "📢 {title}", topicDec.Template)

				// Verify icons
				iconPath, exists := icons["test_project"]
				assert.True(t, exists, "icon should exist")
				assert.Equal(t, "/path/to/icon.png", iconPath)
			},
		},
		{
			name: "missing files create defaults",
			setupFiles: func() {
				// Remove all data files
				if err := os.Remove(filepath.Join(tmpDir, UserDeviceMapJSON)); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(tmpDir, DecorationJSON)); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(tmpDir, TopicDecorationJSON)); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(tmpDir, IconsJSON)); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}

				// Ensure files are in allowedFiles
				allowedFiles[UserDeviceMapJSON] = true
				allowedFiles[DecorationJSON] = true
				allowedFiles[TopicDecorationJSON] = true
				allowedFiles[IconsJSON] = true
			},
			validateData: func(t *testing.T) {
				// Verify empty maps were created
				assert.Empty(t, userDeviceMap)
				assert.Empty(t, decorations)
				assert.Empty(t, topicDecorations)
				assert.Empty(t, icons)

				// Verify files were created
				assert.FileExists(t, filepath.Join(tmpDir, UserDeviceMapJSON))
				assert.FileExists(t, filepath.Join(tmpDir, DecorationJSON))
				assert.FileExists(t, filepath.Join(tmpDir, TopicDecorationJSON))
				assert.FileExists(t, filepath.Join(tmpDir, IconsJSON))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			userDeviceMap = make(map[string]map[string][]string)
			decorations = make(map[string]map[string]Decoration)
			topicDecorations = make(map[string]TopicDecoration)
			icons = make(map[string]string)

			tt.setupFiles()
			loadDataFiles()

			if tt.validateData != nil {
				tt.validateData(t)
			}
		})
	}
}

func TestMainConfigLoading(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name        string
		setupConfig func()
		expectError bool
		validate    func(*testing.T)
	}{
		{
			name: "valid config",
			setupConfig: func() {
				config = Config{
					Projects: map[string]ProjectConfig{
						"test_project": {
							VapidPublicKey: "test-key",
							FirebaseConfig: FirebaseConfig{
								ProjectID: "test-project",
							},
						},
					},
					TrustedProxies: "127.0.0.1/32",
				}
				writeTestJSON(t, filepath.Join(tmpDir, ConfigJSON), config)
				allowedFiles[ConfigJSON] = true
			},
			expectError: false,
			validate: func(t *testing.T) {
				projectConfig, exists := config.Projects["test_project"]
				require.True(t, exists)
				assert.Equal(t, "test-key", projectConfig.VapidPublicKey)
				assert.Equal(t, "test-project", projectConfig.FirebaseConfig.ProjectID)
				assert.Equal(t, "127.0.0.1/32", config.TrustedProxies)
			},
		},
		{
			name: "missing config file",
			setupConfig: func() {
				if err := os.Remove(filepath.Join(tmpDir, ConfigJSON)); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				allowedFiles[ConfigJSON] = true
			},
			expectError: true,
		},
		{
			name: "invalid config json",
			setupConfig: func() {
				err := os.WriteFile(filepath.Join(tmpDir, ConfigJSON), []byte("invalid json"), defaultFileMode)
				require.NoError(t, err)
				allowedFiles[ConfigJSON] = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupConfig()

			if tt.expectError {
				assert.Error(t, loadJSON(ConfigJSON, &config))
			} else {
				err := loadJSON(ConfigJSON, &config)
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t)
				}
			}
		})
	}
}
