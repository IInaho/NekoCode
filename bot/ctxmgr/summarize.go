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

// BuildPrompt assembles a structured summarization prompt from messages.
// Emulates opencode's compaction: incremental update of structured summary
// with Goal / Progress / Key Decisions / Next Steps / Critical Context / Relevant Files.
func BuildPrompt(msgs []llm.Message, prevSummary string) string {
	var b strings.Builder
	for _, m := range msgs {
		limit := 500
		if m.Role == "tool" {
			limit = 800 // tool results carry more signal
		}
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, truncateStr(m.Content, limit))
	}
	conversation := b.String()

	template := `You are an anchored context summarization assistant for coding sessions.
Summarize only the conversation history provided below.
If a previous summary exists, update it incrementally — add new information and remove superseded items.
Do NOT mention that you are summarizing or compacting context.
Keep each section concise.

Output in this exact format:

[目标]
用户正在完成的目标是什么（1-2句话）

[进展]
已完成：已解决或已完成的步骤
进行中：正在进行的工作
阻塞：当前遇到的阻塞或障碍

[关键决策]
重要的技术选型、架构决策或妥协方案

[下一步]
接下来应当执行的操作步骤

[关键上下文]
用户偏好、约束条件、环境信息等必须记住的上下文

[相关文件]
按重要性排列的关键文件路径列表，标注其作用`

	if prevSummary != "" {
		return fmt.Sprintf("%s\n\n[当前摘要]\n%s\n\n[新对话]\n%s\n\n[更新后的结构化摘要]:",
			template, prevSummary, conversation)
	}
	return fmt.Sprintf("%s\n\n[对话]\n%s\n\n[结构化摘要]:", template, conversation)
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
