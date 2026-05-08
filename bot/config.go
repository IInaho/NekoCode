// config.go — 配置加载（~/.primusbot/config.json）+ 斜杠命令系统。
package bot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// --- Commands ---

type Command struct {
	Name string
	Args []string
	Raw  string
}

type CommandHandler func(cmd *Command) (string, bool)

type Parser struct {
	handlers map[string]CommandHandler
}

func NewParser() *Parser {
	return &Parser{handlers: make(map[string]CommandHandler)}
}

func (p *Parser) Register(name string, handler CommandHandler) {
	p.handlers[name] = handler
}

func (p *Parser) Commands() []string {
	names := make([]string, 0, len(p.handlers))
	for name := range p.handlers {
		names = append(names, name)
	}
	return names
}

func (p *Parser) Parse(input string) *Command {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return &Command{Name: "", Raw: input}
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return &Command{Name: "", Raw: input}
	}
	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}
	return &Command{Name: name, Args: args, Raw: input}
}

func (p *Parser) Execute(cmd *Command) (string, bool) {
	if cmd.Name == "" {
		return "", false
	}
	handler, exists := p.handlers[cmd.Name]
	if !exists {
		return "Unknown command: /" + cmd.Name + ". Type /help for available commands.", true
	}
	return handler(cmd)
}

func RegisterDefaultCommands(p *Parser, callbacks *CommandCallbacks) {
	p.Register("help", func(cmd *Command) (string, bool) {
		return `Available commands:
  /help        Show this help message
  /clear       Clear conversation history
  /stats       Show context stats (messages, tokens, summary)
  /summarize   Force context compression now
  /config      Show current provider and model
`, true
	})

	p.Register("clear", func(cmd *Command) (string, bool) {
		if callbacks.ClearHistory != nil {
			callbacks.ClearHistory()
		}
		return "Conversation history cleared.", true
	})

	p.Register("stats", func(cmd *Command) (string, bool) {
		if callbacks.ContextStats != nil {
			return callbacks.ContextStats(), true
		}
		return "Stats unavailable", true
	})

	p.Register("summarize", func(cmd *Command) (string, bool) {
		if callbacks.ForceSummarize != nil {
			result, err := callbacks.ForceSummarize()
			if err != nil {
				return "Summarize failed: " + err.Error(), true
			}
			return result, true
		}
		return "Summarize unavailable", true
	})

	p.Register("config", func(cmd *Command) (string, bool) {
		if callbacks.GetConfig != nil {
			return callbacks.GetConfig(), true
		}
		return "", true
	})
}

type CommandCallbacks struct {
	ClearHistory   func()
	GetConfig      func() string
	ForceSummarize func() (string, error)
	ContextStats   func() string
}
