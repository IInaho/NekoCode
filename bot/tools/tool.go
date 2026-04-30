// Package tools 定义工具接口和注册机制。Tool 是所有工具的抽象，
// Registry 管理工具集合，Descriptor 生成 LLM 可用的工具描述，
// DangerLevel 分级安全控制，ParseCall 解析工具调用协议。
package tools

import (
	"context"
	"fmt"
	"strings"
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
		return "write"
	case LevelDestructive:
		return "destructive"
	case LevelForbidden:
		return "forbidden"
	default:
		return "unknown"
	}
}

type Tool interface {
	Name() string
	Description() string
	Parameters() []Parameter
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

func DescriptorOf(t Tool) Descriptor {
	return Descriptor{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.Parameters(),
	}
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
		descs = append(descs, DescriptorOf(t))
	}
	return descs
}

// ParseCall parses a tool call string in the format "toolName:key1=val1,key2=val2".
func ParseCall(input string) (name string, args map[string]interface{}, err error) {
	if input == "" {
		return "", nil, fmt.Errorf("empty tool call")
	}

	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid tool call format: %s", input)
	}

	name = strings.TrimSpace(parts[0])
	args = make(map[string]interface{})

	argsStr := strings.TrimSpace(parts[1])
	if argsStr == "" {
		return name, args, nil
	}

	for _, pair := range strings.Split(argsStr, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			args[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return name, args, nil
}
