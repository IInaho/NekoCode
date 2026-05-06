// 配置管理：从 ~/.primusbot/config.json 加载 provider / api_key / model / base_url。
// 文件不存在时返回默认配置（openai / gpt-4）。
package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
