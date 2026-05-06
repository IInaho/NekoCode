package ctxmgr

import (
	"fmt"
	"strings"

	"primusbot/llm"
)

// NeedsSummarization returns true when messages should be compressed.
func (m *Manager) NeedsSummarization() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.summarizer == nil || len(m.messages) <= m.windowSize {
		return false
	}
	return m.estimatedTokens() > m.tokenBudget/2
}

// Summarize compresses the oldest messages via the configured summarizer.
func (m *Manager) Summarize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.summarizer == nil {
		return nil
	}

	keep := m.windowSize / 2
	if len(m.messages) <= keep {
		return nil
	}

	split := len(m.messages) - keep
	toSummarize := make([]llm.Message, split)
	copy(toSummarize, m.messages[:split])

	newSummary, err := m.summarizer(toSummarize, m.summary)
	if err != nil {
		return fmt.Errorf("summarize: %w", err)
	}

	m.summary = newSummary
	m.messages = m.messages[split:]
	return nil
}

// BuildPrompt assembles a summarization prompt from messages.
func BuildPrompt(msgs []llm.Message, prevSummary string) string {
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, truncateStr(m.Content, 500))
	}
	conversation := b.String()

	if prevSummary != "" {
		return fmt.Sprintf(
			"请更新以下对话摘要（合并新旧信息），保留关键事实和用户偏好，不超过300字。\n\n[当前摘要]\n%s\n\n[新对话]\n%s\n\n[更新后的摘要]:",
			prevSummary, conversation,
		)
	}
	return fmt.Sprintf(
		"请将以下对话总结为简洁摘要，保留关键事实和用户偏好，不超过300字。\n\n%s\n\n[摘要]:",
		conversation,
	)
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}
