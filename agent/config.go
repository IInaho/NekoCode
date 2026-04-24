package agent

import "time"

type Config struct {
	MaxIterations int           `json:"max_iterations"`
	Timeout       time.Duration `json:"timeout"`
	EnableTools   bool          `json:"enable_tools"`
	SystemPrompt  string        `json:"system_prompt"`
}

var DefaultConfig = Config{
	MaxIterations: 10,
	Timeout:       60 * time.Second,
	EnableTools:   true,
}

type Option func(*Config)

func WithMaxIterations(n int) Option {
	return func(c *Config) {
		c.MaxIterations = n
	}
}

func WithTimeout(t time.Duration) Option {
	return func(c *Config) {
		c.Timeout = t
	}
}

func WithSystemPrompt(p string) Option {
	return func(c *Config) {
		c.SystemPrompt = p
	}
}

func NewConfig(opts ...Option) *Config {
	cfg := DefaultConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return &cfg
}
