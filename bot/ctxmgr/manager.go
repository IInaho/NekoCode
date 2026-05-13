package ctxmgr

import (
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
	summary         string
	compactBoundary int            // messages before this index are behind the summary
	windowSize      int
	tokenBudget     int
	summarizer      Summarizer
	compactCount    int            // cumulative tool results cleared by microCompact
	trimCount       int            // cumulative messages permanently discarded by trim
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
		windowSize:     defaultWindowSize,
		tokenBudget:    defaultTokenBudget,
		tokenTracker:   &TokenTracker{},
		autoCompactCfg: DefaultAutoCompactConfig,
		anchor:         &Anchor{},
	}
}

func (m *Manager) SetSummarizer(fn Summarizer)       { m.summarizer = fn }
func (m *Manager) SetSystemPrompt(s string)          { m.mu.Lock(); defer m.mu.Unlock(); m.systemPrompt = s }
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

// TrimCount returns cumulative messages permanently discarded by trim.
func (m *Manager) TrimCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.trimCount
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

// ForceCompact aggressively compacts context for emergency use (e.g., when the
// primary LLM call has already failed and we need a last-resort summary).
// Clears ALL compactable tool results and forces summarization.
func (m *Manager) ForceCompact() {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Clear all compactable tool results (no minimum keep).
	var compacted int
	for i, msg := range m.messages {
		if msg.Role == "tool" && msg.Content != clearedMarker && m.isCompactableResult(i) {
			m.messages[i].Content = clearedMarker
			compacted++
		}
	}
	m.compactCount += compacted
}

// BuildMinimal builds a minimal context for emergency synthesis.
// Only includes system prompt, anchor, and the last few user/assistant turns.
func (m *Manager) BuildMinimal() []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]llm.Message, 0, 6)

	if m.systemPrompt != "" {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	if m.anchor != nil {
		if at := m.anchor.BuildAnchor(); at != "" {
			out = append(out, llm.Message{Role: "system", Content: at})
		}
	}

	// Take only the last 3 user messages and their assistant/tool responses.
	targetUsers := 3
	userCount := 0
	tailStart := len(m.messages)
	boundary := m.compactBoundary
	if boundary > 0 && m.summary != "" && boundary < len(m.messages) {
		// Skip archived messages, but include the summary.
		out = append(out, llm.Message{Role: "system", Content: "[Summary]\n" + m.summary})
	} else {
		boundary = 0
	}
	for i := len(m.messages) - 1; i >= boundary; i-- {
		if m.messages[i].Role == "user" {
			userCount++
			if userCount >= targetUsers {
				tailStart = i
				break
			}
		}
	}
	if tailStart < boundary {
		tailStart = boundary
	}
	// Collect messages from tailStart onward, filtering both orphaned tool
	// results AND assistant messages whose tool results are missing (Anthropic
	// rejects assistant(tool_use) blocks without matching tool_result blocks).
	msgs := m.messages[tailStart:]

	for _, msg := range m.filterValidMessages(msgs) {
		if msg.Content != clearedMarker {
			out = append(out, msg)
		}
	}
	return out
}

// RemoveMessages removes a range of messages by index.
// Used for internal cleanup (e.g., clearing stale skill context).
func (m *Manager) RemoveMessages(startIdx, endIdx int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if startIdx < 0 || endIdx >= len(m.messages) || startIdx > endIdx {
		return
	}
	n := endIdx - startIdx + 1
	m.messages = append(m.messages[:startIdx], m.messages[endIdx+1:]...)
	if m.compactBoundary > startIdx {
		if m.compactBoundary <= endIdx {
			m.compactBoundary = startIdx
		} else {
			m.compactBoundary -= n
		}
	}
}

// FreshStart clears all messages but preserves the summary and system prompt.
func (m *Manager) FreshStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make([]llm.Message, 0)
	m.compactBoundary = 0
	m.todoText = ""
	m.tokenTracker = &TokenTracker{}
	m.anchor = &Anchor{}
}

// Build assembles messages for an LLM call.
//
// Message ordering is designed for prefix caching (server-side KV cache reuse).
// Static content comes first (system prompt, anchor), then message history
// (stable prefix between turns), then dynamic content (todo, skills, summary)
// which may change between turns. This maximizes the cacheable prefix.
func (m *Manager) Build(withTools bool) []llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := m.buildStaticPrefix()

	// Phase 2: message history — trim, filter orphans, apply token budget.
	kept := m.applyWindowAndBudget(out, withTools)
	filtered := m.filterValidMessages(kept)
	out = append(out, filtered...)

	// Phase 3: dynamic suffix — placed after history to preserve cacheable prefix.
	out = append(out, m.buildDynamicSuffix(withTools)...)
	return out
}

// buildStaticPrefix returns the static prefix messages (system prompt + anchor).
func (m *Manager) buildStaticPrefix() []llm.Message {
	out := make([]llm.Message, 0, 2)
	if m.systemPrompt != "" {
		out = append(out, llm.Message{Role: "system", Content: m.systemPrompt})
	}
	if m.anchor != nil {
		if anchorText := m.anchor.BuildAnchor(); anchorText != "" {
			out = append(out, llm.Message{Role: "system", Content: anchorText})
		}
	}
	return out
}

// applyWindowAndBudget applies the sliding window and token budget to message history.
// Returns the trimmed messages slice.
func (m *Manager) applyWindowAndBudget(staticPrefix []llm.Message, withTools bool) []llm.Message {
	kept := m.messages
	if m.compactBoundary > 0 && m.summary != "" && m.compactBoundary < len(m.messages) {
		kept = m.messages[m.compactBoundary:]
	}
	if len(kept) > m.windowSize {
		kept = kept[len(kept)-m.windowSize:]
	}

	// Reserve budget for static prefix + dynamic suffix.
	reserved := estimateTokens(staticPrefix) + m.dynamicSuffixTokens(withTools)
	budget := m.tokenBudget - reserved - tokenOverhead(withTools)

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
	return kept
}

// filterValidMessages removes orphaned tool results and assistant messages with
// missing tool results. Anthropic API rejects assistant(tool_use) blocks without
// matching tool_result blocks.
func (m *Manager) filterValidMessages(kept []llm.Message) []llm.Message {
	// Pass 1: identify which assistant messages have all tool results present.
	hasToolResult := make(map[string]bool)
	for _, msg := range kept {
		if msg.Role == "tool" && msg.ToolCallID != "" && msg.Content != clearedMarker {
			hasToolResult[msg.ToolCallID] = true
		}
	}
	validAssistantIdx := make(map[int]bool)
	for i, msg := range kept {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			allPresent := true
			for _, tc := range msg.ToolCalls {
				if tc.ID != "" && !hasToolResult[tc.ID] {
					allPresent = false
					break
				}
			}
			if allPresent {
				validAssistantIdx[i] = true
			}
		}
	}

	validIDs := make(map[string]bool)
	filtered := make([]llm.Message, 0, len(kept))
	for i, msg := range kept {
		m := msg
		if m.Content == "" && m.Role != "system" {
			m.Content = "."
		}
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			if !validAssistantIdx[i] {
				continue
			}
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
		filtered = append(filtered, m)
	}
	return filtered
}

// dynamicSuffixTokens estimates tokens consumed by the dynamic suffix.
func (m *Manager) dynamicSuffixTokens(withTools bool) int {
	n := 0
	if withTools {
		n += 200 // closing instruction
	}
	if m.todoText != "" {
		n += estimateString(m.todoText) + 20
	}
	return n
}

// buildDynamicSuffix returns the dynamic suffix messages (todo, skills, summary, tool hint).
func (m *Manager) buildDynamicSuffix(withTools bool) []llm.Message {
	var out []llm.Message
	if m.todoText != "" {
		out = append(out, llm.Message{Role: "system", Content: "[Task progress]\n" + m.todoText})
	}
	if m.skillList != "" {
		out = append(out, llm.Message{Role: "system", Content: m.skillList})
	}
	if m.summary != "" {
		out = append(out, llm.Message{Role: "system", Content: "[Summary]\n" + m.summary})
	}
	if withTools {
		out = append(out, llm.Message{
			Role:    "system",
			Content: "When the user asks you to perform actions, select the right tool: edit to modify files, grep to search content, glob to find files, read to read files, write to create files, list to list directories, todo_write to track tasks, bash to run commands, task to delegate complex work to sub-agents. You MUST actually invoke tools — don't just describe what to do. If the user is just chatting, reply directly.",
		})
	}
	return out
}
