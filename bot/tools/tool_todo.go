package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TodoItem represents a single task in the agent's todo list.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // "pending", "in_progress", "completed"
}

// TodoFunc is called whenever the todo list is updated.
type TodoFunc func(items []TodoItem)

type TodoWriteTool struct {
	mu       sync.Mutex
	onUpdate TodoFunc
	items    []TodoItem
}

func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{}
}

// SetUpdateFn wires the TUI callback. Thread-safe, can be called after registration.
func (t *TodoWriteTool) SetUpdateFn(fn TodoFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onUpdate = fn
}

func (t *TodoWriteTool) Name() string                                   { return "todo_write" }
func (t *TodoWriteTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeSequential }
func (t *TodoWriteTool) DangerLevel(map[string]interface{}) DangerLevel    { return LevelSafe }
func (t *TodoWriteTool) Description() string {
	return `更新任务列表（仅记录，不负责规划）。每次调用完全替换列表。一次性写入完整列表，禁止多次追加。格式: [{"content":"...","status":"pending|in_progress|completed"}]`
}

func (t *TodoWriteTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "todos", Type: "string", Required: true, Description: "JSON 格式的任务列表：[{\"content\":\"...\",\"status\":\"pending|in_progress|completed\"}]"},
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// Accept both JSON string and native array (LLM function calling may pass either).
	var items []TodoItem
	switch v := args["todos"].(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("缺少 todos 参数")
		}
		if err := json.Unmarshal([]byte(v), &items); err != nil {
			return "", fmt.Errorf("解析 todos 失败: %v", err)
		}
	case []interface{}:
		raw, _ := json.Marshal(v)
		if err := json.Unmarshal(raw, &items); err != nil {
			return "", fmt.Errorf("解析 todos 失败: %v", err)
		}
	default:
		return "", fmt.Errorf("todos 参数格式错误：需要 JSON 字符串或数组，收到 %T", args["todos"])
	}

	t.mu.Lock()
	t.items = items
	fn := t.onUpdate
	t.mu.Unlock()

	if fn != nil {
		fn(items)
	}

	// Format a human-readable summary for the agent's context.
	var b strings.Builder
	fmt.Fprintf(&b, "任务列表已更新 (%d 项):\n", len(items))
	for i, it := range items {
		icon := map[string]string{
			"pending":    "⬜",
			"in_progress": "🔄",
			"completed":  "✅",
		}[it.Status]
		if icon == "" {
			icon = "⬜"
		}
		fmt.Fprintf(&b, "%d. %s %s\n", i+1, icon, it.Content)
	}
	return b.String(), nil
}
