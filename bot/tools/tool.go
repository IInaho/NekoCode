// Package tools 定义工具接口和注册机制。Tool 是所有工具的抽象，
// Registry 管理工具集合，Descriptor 生成 LLM 可用的工具描述，
// DangerLevel 分级安全控制。
package tools

import (
	"context"
	"fmt"
	"sync"
)

type DangerLevel int

const (
	LevelSafe        DangerLevel = iota // read-only, auto-approve
	LevelWrite                          // file modification, confirm
	LevelDestructive                    // deletion, critical changes, confirm
	LevelForbidden                      // never allow
)

func (d DangerLevel) String() string {
	switch d {
	case LevelSafe:
		return "safe"
	case LevelWrite:
		return "modify"
	case LevelDestructive:
		return "danger"
	case LevelForbidden:
		return "blocked"
	default:
		return "unknown"
	}
}

type ExecutionMode int

const (
	ModeParallel   ExecutionMode = iota // can run concurrently with other tools
	ModeSequential                      // must run alone, blocks other tools
)

type ToolCallItem struct {
	ID   string
	Name string
	Args map[string]interface{}
}

type ToolCallResult struct {
	ID     string
	Name   string
	Output string
	Error  string
}

type Tool interface {
	Name() string
	Description() string
	Parameters() []Parameter
	ExecutionMode(args map[string]interface{}) ExecutionMode
	DangerLevel(args map[string]interface{}) DangerLevel
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

type Parameter struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

type Descriptor struct {
	Name        string
	Description string
	Parameters  []Parameter
}

type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return t, nil
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	descs := make([]Descriptor, 0, len(r.tools))
	for _, t := range r.tools {
		descs = append(descs, Descriptor{t.Name(), t.Description(), t.Parameters()})
	}
	return descs
}

func RegisterDefaults(r *Registry) {
	r.Register(&BashTool{})
	r.Register(&ReadTool{})
	r.Register(&WriteTool{})
	r.Register(&ListTool{})
	r.Register(&GlobTool{})
	r.Register(&EditTool{})
	r.Register(&GrepTool{})
	r.Register(NewWebSearchTool())
	r.Register(NewWebFetchTool())
	r.Register(NewTodoWriteTool())
	r.Register(NewTaskTool())
	r.Register(NewSnipTool())
}
