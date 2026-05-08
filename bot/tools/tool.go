// Package tools 定义工具接口和注册机制。Tool 是所有工具的抽象，
// Registry 管理工具集合，Descriptor 生成 LLM 可用的工具描述，
// DangerLevel 分级安全控制，ParseCall 解析工具调用协议。
package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
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

// ParseCall parses a tool call string in the format "toolName:key1=val1,key2=val2".
// Values containing commas or spaces may be double-quoted. Backslash escapes are
// supported inside quoted values (\\ for backslash, \" for literal quote).
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

	for _, pair := range SplitPairs(argsStr) {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			args[strings.TrimSpace(kv[0])] = unquote(strings.TrimSpace(kv[1]))
		}
	}

	return name, args, nil
}

// SplitPairs splits on commas that are not inside double-quoted segments.
func SplitPairs(s string) []string {
	var pairs []string
	start := 0
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '\\':
			if inQuote && i+1 < len(s) {
				i++ // skip escaped char
			}
		case ',':
			if !inQuote {
				pairs = append(pairs, s[start:i])
				start = i + 1
			}
		}
	}
	pairs = append(pairs, s[start:])
	return pairs
}

// unquote strips surrounding double-quotes and resolves backslash escapes.
func unquote(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	var b strings.Builder
	inner := s[1 : len(s)-1]
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\\' && i+1 < len(inner) {
			b.WriteByte(inner[i+1])
			i++
		} else {
			b.WriteByte(inner[i])
		}
	}
	return b.String()
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
}

var ansiRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// StripAnsi removes ANSI escape sequences from a string.
func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

var toolTransport = &http.Transport{
	MaxIdleConns:    10,
	IdleConnTimeout: 60 * time.Second,
}

func NewToolHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: toolTransport,
		Timeout:   timeout,
	}
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func TruncateByRune(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// validatePath resolves path against the current working directory and rejects
// paths that escape via ".." traversal.
func validatePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("路径解析失败: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return abs, nil // can't validate, trust the path
	}
	rel, err := filepath.Rel(cwd, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("路径超出工作目录范围: %s", path)
	}
	return abs, nil
}
