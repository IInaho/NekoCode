package agent

import (
	"fmt"
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
	workingMemory    map[string]interface{}
	persistentMemory []MemoryItem
	mu               sync.RWMutex
}

func NewMemory() *Memory {
	return &Memory{
		workingMemory:    make(map[string]interface{}),
		persistentMemory: make([]MemoryItem, 0),
	}
}

func (m *Memory) Set(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workingMemory[key] = value
}

func (m *Memory) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.workingMemory[key]
	return v, ok
}

func (m *Memory) Add(item MemoryItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.persistentMemory = append(m.persistentMemory, item)
}

func (m *Memory) GetHistory() []MemoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]MemoryItem, len(m.persistentMemory))
	copy(result, m.persistentMemory)
	return result
}

func (m *Memory) Last() *MemoryItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.persistentMemory) == 0 {
		return nil
	}
	return &m.persistentMemory[len(m.persistentMemory)-1]
}

func (m *Memory) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workingMemory = make(map[string]interface{})
	m.persistentMemory = make([]MemoryItem, 0)
}

func (m *Memory) Summary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var summary string
	for i, item := range m.persistentMemory {
		summary += fmt.Sprintf("Step %d: %s -> %s\n  Output: %s\n",
			i+1, item.Thought, item.Action, truncate(item.Output, 100))
	}
	if summary == "" {
		return "无历史记录"
	}
	return summary
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
