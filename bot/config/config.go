package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Provider       string `json:"provider"`
	APIKey         string `json:"api_key"`
	Model          string `json:"model"`
	BaseURL        string `json:"base_url"`
	TokenBudget    int    `json:"token_budget"`
	ThinkingBudget int    `json:"thinking_budget"` // 0=default, -1=off; Anthropic only
}

var Default = Config{
	Provider:       "openai",
	Model:          "gpt-4",
	BaseURL:        "https://api.openai.com/v1",
	TokenBudget:    128000,
	ThinkingBudget: 16000,
}

func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &Default, nil
	}

	configPath := filepath.Join(homeDir, ".nekocode", "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return &Default, nil
	}

	cfg := Default
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Default, nil
	}

	return &cfg, nil
}
