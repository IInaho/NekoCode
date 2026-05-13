// session/memory.go — Session Memory file I/O.
//
// Provides read access to the session memory Markdown file used as a free
// summary source for the /new command and SummarizeWithSessionMemory.
package session

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed memory_template.md
var DefaultTemplate string

// Memory manages the session memory file.
type Memory struct {
	path     string
	template string
}

func New(sessionID, template string) (*Memory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	dir := filepath.Join(home, ".nekocode", "sessions", sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("session memory: mkdir: %w", err)
	}
	if template == "" {
		template = DefaultTemplate
	}
	path := filepath.Join(dir, "memory.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, []byte(template), 0644); err != nil {
			return nil, fmt.Errorf("session memory: write template: %w", err)
		}
	}
	return &Memory{
		path:     path,
		template: template,
	}, nil
}

// ReadContent returns the current memory file content.
func (m *Memory) ReadContent() string {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return ""
	}
	return string(data)
}

// HasSubstance returns true if the memory file differs from the empty template.
func (m *Memory) HasSubstance() bool {
	c := m.ReadContent()
	return c != "" && c != m.template
}
