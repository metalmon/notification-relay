package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	configPath string
	// Whitelist of allowed configuration files
	allowedFiles = map[string]bool{
		ConfigJSON:        true,
		CredentialsJSON:   true,
		UserDeviceMapJSON: true,
		DecorationJSON:    true,
		IconsJSON:         true,
	}
)

func getConfigPath(filename string) string {
	// Check if file is in whitelist
	if !allowedFiles[filename] {
		panic(fmt.Sprintf("Unauthorized file access attempt: %s", filename))
	}

	dir := filepath.Dir(configPath)
	// Clean the path and ensure it's within the config directory
	fullPath := filepath.Join(dir, filename)
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, dir) {
		panic(fmt.Sprintf("Invalid file path: %s (attempting to access outside config directory)", filename))
	}
	return cleanPath
}

func init() {
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

func loadDataFiles() {
	// Skip credentials as they are already loaded by initCredentials()
	ensureFileExists(UserDeviceMapJSON, &userDeviceMap)
	ensureFileExists(DecorationJSON, &decorations)
	ensureFileExists(IconsJSON, &icons)
}

func ensureFileExists(filename string, defaultValue interface{}) {
	fullPath := getConfigPath(filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			log.Fatalf("Failed to create directory for %s: %v", filename, err)
		}
		// Save default value and check for errors
		if err := saveJSON(fullPath, defaultValue); err != nil {
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

	file, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, v)
}

func saveJSON(filename string, v interface{}) error {
	fullPath := getConfigPath(filename)
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal JSON: %v", err))
	}
	return os.WriteFile(fullPath, data, 0600)
}
