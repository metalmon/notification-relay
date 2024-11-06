package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

var (
	configPath string
)

func init() {
	// Check for config path in environment
	configPath = os.Getenv("NOTIFICATION_RELAY_CONFIG")
	if configPath == "" {
		// Default paths in order of preference
		paths := []string{
			"./config.json",
			"/etc/notification-relay/config.json",
		}
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
	}
	if configPath == "" {
		log.Fatal("No config file found")
	}
}

func basicAuth() gin.HandlerFunc {
	return gin.BasicAuth(gin.Accounts{
		config.APIKey: config.APISecret,
	})
}

func loadDataFiles() {
	ensureFileExists("user-device-map.json", &userDeviceMap)
	ensureFileExists("decoration.json", &decorations)
	ensureFileExists("icons.json", &icons)
}

func getConfigPath(filename string) string {
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, filename)
}

func ensureFileExists(filename string, defaultValue interface{}) {
	fullPath := getConfigPath(filename)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		saveJSON(fullPath, defaultValue)
	}
}

func loadJSON(filename string, v interface{}) error {
	fullPath := getConfigPath(filename)
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
		return err
	}
	return os.WriteFile(fullPath, data, 0644)
} 