package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	BaseURL  string `json:"base_url"`
}

var DefaultConfig = Config{
	Provider: "openai",
	Model:    "gpt-4",
	BaseURL:  "https://api.openai.com/v1",
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &DefaultConfig, nil
	}

	configPath := filepath.Join(homeDir, ".primusbot", "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return &DefaultConfig, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &DefaultConfig, nil
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(homeDir, ".primusbot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}
