package ctxmgr

import (
	"fmt"
	"primusbot/llm"
	"strings"
	"sync"
)

type Summarizer func(msgs []llm.Message, prevSummary string) (string, error)

type Manager struct {
	mu           sync.RWMutex
	systemPrompt string
	messages     []llm.Message
	summary      string
	windowSize   int
	tokenBudget  int
	summarizer   Summarizer
}

const (
	defaultWindowSize  = 20
	defaultTokenBudget = 8000
)

func New(systemPrompt string) *Manager {
	return &Manager{
		systemPrompt: systemPrompt,
		messages:     make([]llm.Message, 0),
		windowSize:   defaultWindowSize,
		tokenBudget:  defaultTokenBudget,
	}
}

func (m *Manager) SetSummarizer(fn Summarizer) {
	m.summarizer = fn
}

func (m *Manager) Add(role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, llm.Message{Role: role, Content: content})
}

func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]llm.Message, 0)
	m.summary = ""
}

func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// Stats returns (messageCount, estimatedTokens, hasSummary).
func (m *Manager) Stats() (int, int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := m.allMessages()
	return len(m.messages), estimateTokens(all), m.summary != ""
}

// TokenUsage returns (estimatedTokens, budget).
func (m *Manager) TokenUsage() (int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return estimateTokens(m.allMessages()), m.tokenBudget
}

// NeedsSummarization returns true when messages should be compressed.
func (m *Manager) NeedsSummarization() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.summarizer == nil || len(m.messages) <= m.windowSize {
		return false
	}
	return estimateTokens(m.messages) > m.tokenBudget/2
}

// Summarize compresses the oldest messages via the configured summarizer.
func (m *Manager) Summarize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.summarizer == nil {
		return nil
	}

	// Keep at least half the window as recent context
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

// Build assembles messages for an LLM call.
// Uses token budget: keeps system prompt, summary, then fills with recent messages.
func (m *Manager) Build(withTools bool) []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]llm.Message, 0, len(m.messages)+3)

	if m.systemPrompt != "" {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}

	if m.summary != "" {
		out = append(out, llm.Message{
			Role:    "system",
			Content: "[历史摘要]\n" + m.summary,
		})
	}

	// Recent messages within token budget
	kept := m.messages
	if len(kept) > m.windowSize {
		kept = kept[len(kept)-m.windowSize:]
	}

	used := estimateTokens(out)
	budget := m.tokenBudget - used - tokenOverhead(withTools)

	// Drop oldest pairs until within budget (keep at least last 2)
	for len(kept) > 2 {
		if estimateTokens(kept) <= budget {
			break
		}
		kept = kept[2:] // drop one user+assistant pair
	}

	out = append(out, kept...)

	if withTools {
		out = append(out, llm.Message{
			Role:    "system",
			Content: "当用户要求你执行具体操作（读写文件、运行命令、搜索文件等）时，必须调用函数来实际完成任务。不要只描述你要做什么。如果用户只是在闲聊，直接回复即可。",
		})
	}

	return out
}

// allMessages returns system prompt + summary + all messages (for token estimation only).
func (m *Manager) allMessages() []llm.Message {
	out := make([]llm.Message, 0, len(m.messages)+2)
	if m.systemPrompt != "" {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	if m.summary != "" {
		out = append(out, llm.Message{Role: "system", Content: m.summary})
	}
	out = append(out, m.messages...)
	return out
}

func estimateTokens(msgs []llm.Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Role) + len(m.Content)
	}
	return n
}

func tokenOverhead(withTools bool) int {
	if withTools {
		return 200 // tool instruction + tool defs overhead
	}
	return 0
}

// BuildPrompt is a convenience helper for the summarizer callback.
func BuildPrompt(msgs []llm.Message, prevSummary string) string {
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, truncate(m.Content, 500))
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
