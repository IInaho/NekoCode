package ctxmgr

import "nekocode/llm"

func (m *Manager) Add(role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, llm.Message{Role: role, Content: content})
	m.tokenTracker.AddNew(len(role) + len(content))

	// Auto-extract critical constraints from user messages.
	if role == "user" && m.anchor != nil {
		for _, c := range m.anchor.ExtractConstraints(content) {
			m.anchor.AddConstraint(c)
		}
	}
}

func (m *Manager) AddAssistantResponse(content, reasoning string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, llm.Message{
		Role:             "assistant",
		Content:          content,
		ReasoningContent: reasoning,
	})
	m.tokenTracker.AddNew(len("assistant") + len(content) + len(reasoning))
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
	m.tokenTracker.AddNew(len("assistant") + len(content) + len(reasoning))
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
	m.tokenTracker.AddNew(len(role) + len(content) + len(toolCallID))
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
		m.tokenTracker.AddNew(len(role) + len(r.Content) + len(r.ToolCallID))
	}
}

func (m *Manager) SetTodos(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.todoText = text
}

func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]llm.Message, 0)
	m.summary = ""
	m.compactBoundary = 0
	m.todoText = ""
}
