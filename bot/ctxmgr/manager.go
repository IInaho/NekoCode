package ctxmgr

import (
	"fmt"
	"sync"

	"nekocode/llm"
)

type Summarizer func(msgs []llm.Message, prevSummary string) (string, error)

type Manager struct {
	mu              sync.RWMutex
	systemPrompt    string
	todoText        string // current todo list, injected into every LLM call
	skillList       string // available skills, injected into every LLM call
	messages        []llm.Message
	snipped         map[int]bool // indices of snipped messages (removed by model)
	summary         string
	compactBoundary int    // messages before this index are behind the summary
	windowSize      int
	tokenBudget     int
	summarizer      Summarizer
	compactCount    int            // cumulative tool results cleared by microCompact
	tokenTracker    *TokenTracker   // accurate token tracking from API responses
	autoCompactCfg  AutoCompactConfig
	anchor          *Anchor        // immutable critical constraints + goal, never compressed
}

const (
	defaultWindowSize  = 20
	defaultTokenBudget = 64000
)

func New(systemPrompt string) *Manager {
	return &Manager{
		systemPrompt:   systemPrompt,
		messages:       make([]llm.Message, 0),
		snipped:        make(map[int]bool),
		windowSize:     defaultWindowSize,
		tokenBudget:    defaultTokenBudget,
		tokenTracker:   &TokenTracker{},
		autoCompactCfg: DefaultAutoCompactConfig,
		anchor:         &Anchor{},
	}
}

func (m *Manager) SetSummarizer(fn Summarizer)       { m.summarizer = fn }
func (m *Manager) SetSummary(s string)               { m.summary = s }
func (m *Manager) SetSkillList(s string)             { m.skillList = s }
func (m *Manager) SetAutoCompactConfig(cfg AutoCompactConfig) { m.autoCompactCfg = cfg }
func (m *Manager) GetAutoCompactConfig() AutoCompactConfig  { return m.autoCompactCfg }
func (m *Manager) RecordUsage(prompt, completion int)        { m.tokenTracker.RecordUsage(prompt, completion) }
func (m *Manager) Anchor() *Anchor                            { return m.anchor }

// AccurateTokens returns token count calibrated against last API usage.
func (m *Manager) AccurateTokens() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Use tracker if it has real data, otherwise fall back to heuristic.
	if t := m.tokenTracker.Total(); t > 0 {
		return t
	}
	return m.estimatedTokens()
}

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

// ToolResultCount returns the number of tool_result messages.
func (m *Manager) ToolResultCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, msg := range m.messages {
		if msg.Role == "tool" {
			n++
		}
	}
	return n
}

// LastAssistantHasToolCall returns true if the last assistant message has tool calls.
func (m *Manager) LastAssistantHasToolCall() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			return len(m.messages[i].ToolCalls) > 0
		}
	}
	return false
}

// Stats returns (messageCount, estimatedTokens, hasSummary).
// Token count reflects only messages that would be sent to the LLM (after
// compactBoundary), not the total archived message count.
func (m *Manager) Stats() (int, int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages), m.visibleEstimatedTokens(), m.summary != ""
}

// visibleEstimatedTokens estimates tokens for the context that Build() would
// actually send — excludes messages archived behind compactBoundary.
// Must be called with the lock held.
func (m *Manager) visibleEstimatedTokens() int {
	visible := m.messages
	if m.compactBoundary > 0 && m.summary != "" && m.compactBoundary < len(m.messages) {
		visible = m.messages[m.compactBoundary:]
	}
	if len(visible) > m.windowSize {
		visible = visible[len(visible)-m.windowSize:]
	}
	return estimateTokens(visible) + estimateTokensSystem(m.systemPrompt, m.summary)
}

// TokenUsage returns (estimatedTokens, budget) for the visible context.
func (m *Manager) TokenUsage() (int, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.visibleEstimatedTokens(), m.tokenBudget
}

// CompactCount returns cumulative tool results cleared by microCompact.
func (m *Manager) CompactCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.compactCount
}

// MicroCompactIfNeeded performs micro-compaction when context is under pressure.
// Only clears when tokens exceed half the budget to avoid losing useful results
// during active exploration (especially list/glob/read that the model depends on).
// Returns the number of tool results cleared.
func (m *Manager) MicroCompactIfNeeded() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	est := m.visibleEstimatedTokens()
	if t := m.tokenTracker.Total(); t > est {
		est = t
	}
	if est < m.tokenBudget/2 {
		return 0
	}
	return m.microCompact()
}

// Snip marks a message range [startIdx, endIdx] as removed.
// Snipped messages are excluded from Build() output.
func (m *Manager) Snip(startIdx, endIdx int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if startIdx < 0 || endIdx >= len(m.messages) || startIdx > endIdx {
		return fmt.Sprintf("snip: indices %d-%d out of range [0, %d]", startIdx, endIdx, len(m.messages)-1)
	}

	snipped := 0
	for i := startIdx; i <= endIdx; i++ {
		if !m.snipped[i] {
			m.snipped[i] = true
			snipped++
		}
	}
	return fmt.Sprintf("Snipped %d messages (indices %d-%d)", snipped, startIdx, endIdx)
}

// FreshStart clears all messages but preserves the summary and system prompt.
func (m *Manager) FreshStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]llm.Message, 0)
	m.snipped = make(map[int]bool)
	m.compactBoundary = 0
	m.todoText = ""
	m.tokenTracker = &TokenTracker{}
	m.anchor = &Anchor{}
}

// Build assembles messages for an LLM call.
func (m *Manager) Build(withTools bool) []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]llm.Message, 0, len(m.messages)+3)

	if m.systemPrompt != "" {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}

	// Immutable anchor: critical constraints + current goal.
	// Never compressed, never evicted. Model sees this before everything.
	if m.anchor != nil {
		if anchorText := m.anchor.BuildAnchor(); anchorText != "" {
			out = append(out, llm.Message{Role: "system", Content: anchorText})
		}
	}

	if m.todoText != "" {
		out = append(out, llm.Message{Role: "system", Content: "[Task progress]\n" + m.todoText})
	}

	if m.skillList != "" {
		out = append(out, llm.Message{Role: "system", Content: m.skillList})
	}

	if m.summary != "" {
		out = append(out, llm.Message{
			Role:    "system",
			Content: "[Summary]\n" + m.summary,
		})
	}

	kept := m.messages
	// Start after compact boundary if a summary exists.
	if m.compactBoundary > 0 && m.summary != "" {
		if m.compactBoundary < len(m.messages) {
			kept = m.messages[m.compactBoundary:]
		}
	}
	if len(kept) > m.windowSize {
		kept = kept[len(kept)-m.windowSize:]
	}

	used := estimateTokens(out) // out already includes system prompt and summary
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

	// Map from original message index (in m.messages) to kept offset.
	origBase := m.compactBoundary
	if len(kept) < len(m.messages) {
		origBase = len(m.messages) - len(kept)
	}

	// Filter: exclude snipped messages, orphans, and inject [id:N] tags.
	validIDs := make(map[string]bool)
	filtered := make([]llm.Message, 0, len(kept))
	for i, msg := range kept {
		origIdx := origBase + i
		if m.snipped[origIdx] {
			continue
		}

		m := msg
		if m.Content == "" && m.Role != "system" {
			m.Content = "."
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				if tc.ID != "" {
					validIDs[tc.ID] = true
				}
			}
		}
		if m.Role == "tool" {
			if m.ToolCallID == "" || !validIDs[m.ToolCallID] {
				continue
			}
		}
		// Inject [id:N] tag on user messages for snip reference.
		// Only when tools are available (snip is a tool).
		if withTools && m.Role == "user" {
			m.Content = m.Content + fmt.Sprintf(" [id:%d]", origIdx)
		}
		filtered = append(filtered, m)
	}
	out = append(out, filtered...)

	if withTools {
		out = append(out, llm.Message{
			Role:    "system",
			Content: "When the user asks you to perform actions, select the right tool: edit to modify files, grep to search content, glob to find files, read to read files, write to create files, list to list directories, todo_write to track tasks, bash to run commands, task to delegate complex work to sub-agents. Use snip to remove old messages no longer needed. You MUST actually invoke tools — don't just describe what to do. If the user is just chatting, reply directly.",
		})
	}

	return out
}
