// todo_list.go — 任务清单组件。
package components

import (
	"sync"

	"nekocode/bot/tools"
)

type TodoList struct {
	mu    sync.Mutex
	items []tools.TodoItem
}

func NewTodoList() *TodoList {
	return &TodoList{}
}

func (t *TodoList) SetItems(items []tools.TodoItem) {
	t.mu.Lock()
	t.items = items
	t.mu.Unlock()
}
