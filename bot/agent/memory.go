// 步骤记忆：记录 Agent 循环中每一步的思考、动作和输出。
// 用于回调展示和后续可扩展的调试/回放。
package agent

import (
	"sync"
	"time"
)

type MemoryItem struct {
	Step      int
	Thought   string
	Action    string
	Output    string
	Timestamp time.Time
}

type Memory struct {
	items []MemoryItem
	mu    sync.RWMutex
}

func NewMemory() *Memory {
	return &Memory{items: make([]MemoryItem, 0)}
}

func (m *Memory) Add(item MemoryItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = append(m.items, item)
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make([]MemoryItem, 0)
}
