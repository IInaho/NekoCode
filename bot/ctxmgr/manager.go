package ctxmgr

import (
	"sync"

	"primusbot/llm"
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
	defaultTokenBudget = 64000
)

func New(systemPrompt string) *Manager {
	return &Manager{
		systemPrompt: systemPrompt,
		messages:     make([]llm.Message, 0),
		windowSize:   defaultWindowSize,
		tokenBudget:  defaultTokenBudget,
	}
}

func (m *Manager) SetSummarizer(fn Summarizer) { m.summarizer = fn }

func (m *Manager) SetTokenBudget(budget int) {
	if budget > 0 {
		m.tokenBudget = budget
	}
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
	return len(m.messages), m.estimatedTokens(), m.summary != ""
}

// TokenUsage returns (estimatedTokens, budget).
func (m *Manager) TokenUsage() (int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.estimatedTokens(), m.tokenBudget
}

// Build assembles messages for an LLM call.
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

	kept := m.messages
	if len(kept) > m.windowSize {
		kept = kept[len(kept)-m.windowSize:]
	}

	used := estimateTokensSystem(m.systemPrompt, m.summary) + estimateTokens(out)
	budget := m.tokenBudget - used - tokenOverhead(withTools)

	for len(kept) > 2 {
		if estimateTokens(kept) <= budget {
			break
		}
		drop := 2
		for drop < len(kept) && kept[drop].Role == "tool" {
			drop++
		}
		kept = kept[drop:]
	}

	// Drop orphaned tool messages at the head.
	for len(kept) > 0 && kept[0].Role == "tool" {
		kept = kept[1:]
	}

	// Filter tool messages referencing unknown tool_call_ids.
	validIDs := make(map[string]bool)
	filtered := make([]llm.Message, 0, len(kept))
	for _, msg := range kept {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" {
					validIDs[tc.ID] = true
				}
			}
		}
		if msg.Role == "tool" {
			if msg.ToolCallID == "" || !validIDs[msg.ToolCallID] {
				continue
			}
		}
		filtered = append(filtered, msg)
	}
	out = append(out, filtered...)

	if withTools {
		out = append(out, llm.Message{
			Role:    "system",
			Content: "当用户要求执行操作时，根据任务选择合适的工具：替换/修改文件内容用 edit，搜索文件内容用 grep，查找文件用 glob，读写文件用 filesystem，运行命令用 bash。必须调用工具实际执行，不要只描述。如果用户只是在闲聊，直接回复即可。",
		})
	}

	return out
}
