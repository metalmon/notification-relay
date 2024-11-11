package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func getConfigPath(filename string) string {
	// Check if file is allowed
	if !allowedFiles[filename] {
		panic(fmt.Sprintf("Unauthorized file access attempt: %s", filename))
	}

	// Get directory from configPath
	configDir := filepath.Dir(configPath)
	return filepath.Join(configDir, filename)
}

var (
	credentials Credentials
)

// Add initialization function that will be called after configPath is set
func initCredentials() {
	// Load credentials from file
	ensureFileExists(CredentialsJSON, make(Credentials))
	if err := loadJSON(CredentialsJSON, &credentials); err != nil {
		log.Fatalf("Failed to load credentials: %v", err)
	}
}

// initConfig initializes the configuration path
func initConfig() {
	// Check for config path in environment
	configPath = os.Getenv("NOTIFICATION_RELAY_CONFIG")
	if configPath == "" {
		// Default paths in order of preference
		paths := []string{
			"./" + ConfigJSON,
			"/etc/notification-relay/" + ConfigJSON,
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
		if configPath == "" {
			log.Fatal("No config file found")
		}
	}
}

func init() {
	initConfig()
}

// loadDataFiles loads all required data files and initializes global variables
func loadDataFiles() {
	// Add files to allowed files map first
	allowedFiles[UserDeviceMapJSON] = true
	allowedFiles[DecorationJSON] = true
	allowedFiles[TopicDecorationJSON] = true
	allowedFiles[IconsJSON] = true

	// Load user device map
	ensureFileExists(UserDeviceMapJSON, make(map[string]map[string][]string))
	if err := loadJSON(UserDeviceMapJSON, &userDeviceMap); err != nil {
		log.Printf("Warning: Failed to load user device map: %v", err)
		userDeviceMap = make(map[string]map[string][]string)
	}

	// Load decorations
	ensureFileExists(DecorationJSON, make(map[string]map[string]Decoration))
	if err := loadJSON(DecorationJSON, &decorations); err != nil {
		log.Printf("Warning: Failed to load decorations: %v", err)
		decorations = make(map[string]map[string]Decoration)
	}

	// Load topic decorations
	ensureFileExists(TopicDecorationJSON, make(map[string]TopicDecoration))
	if err := loadJSON(TopicDecorationJSON, &topicDecorations); err != nil {
		log.Printf("Warning: Failed to load topic decorations: %v", err)
		topicDecorations = make(map[string]TopicDecoration)
	}

	// Load icons
	ensureFileExists(IconsJSON, make(map[string]string))
	if err := loadJSON(IconsJSON, &icons); err != nil {
		log.Printf("Warning: Failed to load icons: %v", err)
		icons = make(map[string]string)
	}
}

func ensureFileExists(filename string, defaultValue interface{}) {
	fullPath := getConfigPath(filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			log.Fatalf("Failed to create directory for %s: %v", filename, err)
		}
		if err := writeJSONToFile(fullPath, defaultValue); err != nil {
			log.Fatalf("Failed to save default value for %s: %v", filename, err)
		}
	}
}

func loadJSON(filename string, v interface{}) error {
	fullPath := getConfigPath(filename)
	// Validate file extension
	if filepath.Ext(fullPath) != ".json" {
		return fmt.Errorf("invalid file extension for %s: must be .json", filename)
	}

	// Use filepath.Clean to sanitize the path
	cleanPath := filepath.Clean(fullPath)
	file, err := os.ReadFile(cleanPath) // #nosec G304 -- path is sanitized
	if err != nil {
		return err
	}
	return json.Unmarshal(file, v)
}

func writeJSONToFile(fullPath string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal JSON: %v", err))
	}
	return os.WriteFile(fullPath, data, 0o600)
}

func saveJSON(filename string, v interface{}) error {
	fullPath := getConfigPath(filename)
	return writeJSONToFile(fullPath, v)
}
