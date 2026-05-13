// todo.go — Todo list types shared between the todo_write tool and the TUI.
package tools

// TodoItem represents a single task in the agent's todo list.
type TodoItem struct {
	Content string `json:"content"`
	Status  string `json:"status"` // "pending", "in_progress", "completed"
}

// TodoFunc is called whenever the todo list is updated.
type TodoFunc func(items []TodoItem)
