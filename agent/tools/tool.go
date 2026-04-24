package tools

import (
	"context"
	"fmt"
	"sync"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() []Parameter
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

type ToolResult struct {
	Success  bool
	Output   string
	Error    string
	Metadata map[string]interface{}
}

type ToolRegistry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *ToolRegistry) AvailableToolsString() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result string
	for name, tool := range r.tools {
		result += fmt.Sprintf("- `%s`: %s\n", name, tool.Description())
	}
	return result
}
