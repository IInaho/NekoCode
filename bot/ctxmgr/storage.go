package ctxmgr

import "primusbot/llm"

func (m *Manager) Add(role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, llm.Message{Role: role, Content: content})
}

func (m *Manager) AddAssistantResponse(content, reasoning string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, llm.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
	})
}

func (m *Manager) AddAssistantToolCall(content, reasoning string, toolCalls []llm.ToolCall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, llm.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
		ToolCalls:        toolCalls,
	})
}

func (m *Manager) AddToolResult(toolCallID, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	role := "tool"
	if toolCallID == "" {
		role = "user"
	}
	m.messages = append(m.messages, llm.Message{
		Role:       role,
		Content:    content,
		ToolCallID: toolCallID,
	})
}

func (m *Manager) AddToolResultsBatch(results []llm.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range results {
		role := "tool"
		if r.ToolCallID == "" {
			role = "user"
		}
		m.messages = append(m.messages, llm.Message{
			Role:       role,
			Content:    r.Content,
			ToolCallID: r.ToolCallID,
		})
	}
}

func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]llm.Message, 0)
	m.summary = ""
}
