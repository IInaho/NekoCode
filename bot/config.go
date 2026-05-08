// config.go — 配置加载（~/.primusbot/config.json）。
package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// --- Config ---

type Config struct {
	Provider    string `json:"provider"`
	APIKey      string `json:"api_key"`
	Model       string `json:"model"`
	BaseURL     string `json:"base_url"`
	TokenBudget int    `json:"token_budget"`
}

var DefaultConfig = Config{
	Provider:    "openai",
	Model:       "gpt-4",
	BaseURL:     "https://api.openai.com/v1",
	TokenBudget: 128000,
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

	cfg := DefaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &DefaultConfig, nil
	}

	return &cfg, nil
}
