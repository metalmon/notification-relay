package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

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

func ensureFileExists(filename string, data interface{}) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(filename), 0755)
		saveJSON(filename, data)
	}
	loadJSON(filename, data)
}

func loadJSON(filename string, v interface{}) error {
	file, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, v)
}

func saveJSON(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
} 