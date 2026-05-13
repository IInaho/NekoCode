package ctxmgr

import (
	"fmt"
	"strings"

	"nekocode/llm"
)

// NeedsSummarization returns true when messages should be compressed.
func (m *Manager) NeedsSummarization() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.summarizer == nil || len(m.messages) <= m.windowSize {
		return false
	}
	// Trigger at 80% since micro-compaction handles tool-result bloat first.
	return m.visibleEstimatedTokens() > m.tokenBudget*8/10
}

// Summarize compresses messages before the compact boundary via the configured
// summarizer. Messages are preserved (not dropped) — the compact boundary is
// updated so Build() only sends post-boundary messages, with the summary
// replacing older content.
func (m *Manager) Summarize() error {
	return m.summarizeInternal(false, "")
}

// SummarizeWithSessionMemory uses pre-extracted session memory content as the
// summary instead of calling the LLM summarizer. This is the "free" summary path.
func (m *Manager) SummarizeWithSessionMemory(smContent string) error {
	if smContent == "" {
		return nil
	}
	return m.summarizeInternal(true, smContent)
}

func (m *Manager) summarizeInternal(useSessionMemory bool, smContent string) error {
	// Phase 1: under lock — prepare data for summarization.
	m.mu.Lock()

	if m.summarizer == nil && !useSessionMemory {
		m.mu.Unlock()
		return nil
	}

	keep := m.windowSize / 2
	if keep < 2 {
		keep = 2
	}

	const preserveTurns = 3
	tailKeep := m.countMessagesForLastNTurns(preserveTurns)
	if tailKeep > keep {
		keep = tailKeep
	}

	if len(m.messages) <= keep {
		m.mu.Unlock()
		return nil
	}

	split := len(m.messages) - keep
	start := m.compactBoundary
	if split <= start {
		m.mu.Unlock()
		return nil
	}

	if useSessionMemory {
		m.summary = smContent
		m.compactBoundary = split
		m.trimOldMessages()
		m.mu.Unlock()
		return nil
	}

	// Snapshot data needed for the LLM call, then release the lock.
	toSummarize := make([]llm.Message, split-start)
	copy(toSummarize, m.messages[start:split])
	prevSummary := m.summary
	anchorConstraints := m.anchor.Constraints()
	m.mu.Unlock()

	// Phase 2: outside lock — LLM call (may take seconds).
	rawSummary, err := m.summarizer(toSummarize, prevSummary)
	if err != nil {
		return fmt.Errorf("summarize: %w", err)
	}

	// Process summary content locally — don't write to m.* without the lock.
	summaryText := FormatCompactSummary(rawSummary)
	if len(strings.TrimSpace(summaryText)) < 50 {
		summaryText = "[Summary unavailable — LLM output malformed]"
	}
	facts := FormatCompactFacts(rawSummary)

	// Quality check: summary must be at least 10% of input or 200 tokens.
	inputTokens := estimateTokens(toSummarize)
	summaryTokens := estimateString(summaryText)
	minSummary := inputTokens / 10
	if minSummary < 200 {
		minSummary = 200
	}

	// Verify critical constraints survived compression.
	// This may trigger a second LLM call — also outside the lock.
	var verifyText string
	if missing := checkConstraints(anchorConstraints, summaryText); len(missing) > 0 {
		verifyPrompt := BuildVerifyPrompt(toSummarize, summaryText, missing)
		if rs, re := m.summarizer([]llm.Message{{Role: "user", Content: verifyPrompt}}, summaryText); re == nil && rs != "" {
			verifyText = FormatCompactSummary(rs)
		}
	}

	// Phase 3: re-acquire lock — apply computed state.
	m.mu.Lock()
	defer m.mu.Unlock()

	m.summary = summaryText
	if verifyText != "" {
		m.summary = verifyText
	}

	if len(facts) > 0 {
		m.anchor.AddFacts(facts)
	}

	if summaryTokens < minSummary {
		keep = keep * 2
		if keep > len(m.messages)-1 {
			keep = len(m.messages) - 1
		}
		split = len(m.messages) - keep
	}

	m.compactBoundary = split
	m.trimOldMessages()
	return nil
}

// trimOldMessages discards messages far before the compact boundary to prevent
// unbounded growth. Must be called with the write lock held.
func (m *Manager) trimOldMessages() {
	const maxPreservedBeforeBoundary = 200
	if m.compactBoundary > maxPreservedBeforeBoundary {
		trim := m.compactBoundary - maxPreservedBeforeBoundary
		m.messages = m.messages[trim:]
		m.compactBoundary -= trim
		m.trimCount += trim
	}
}


func (m *Manager) countMessagesForLastNTurns(n int) int {
	if n <= 0 {
		return 0
	}
	turns := 0
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			turns++
			if turns >= n {
				return len(m.messages) - i
			}
		}
	}
	return len(m.messages) // fewer than n turns total, preserve all
}

// CompactBoundary returns the current compact boundary index.
func (m *Manager) CompactBoundary() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.compactBoundary
}

// NO_TOOLS_PREAMBLE prevents the summarizer model from making tool calls.
// It MUST appear at the TOP of the prompt — models with adaptive-thinking
// are more likely to overlook constraints placed at the end.
//
// Pattern from Claude Code: compact agent has maxTurns=1, so a single
// wasted tool call = zero text output = total compaction failure.
const NO_TOOLS_PREAMBLE = `CRITICAL: Respond with TEXT ONLY. Do NOT call any tools.
- Do NOT use Read, Bash, Grep, Glob, Edit, Write, or ANY other tool.
- Tool calls will be REJECTED and will waste your only turn — you will fail the task.
`

// BuildPrompt assembles a structured summarization prompt from messages.
//
// Uses the dual-block pattern from Claude Code:
//   <analysis> — scratchpad for organizing thoughts (stripped by caller)
//   <summary> — the actual summary that enters context
//
// The 9-section structure preserves all information needed to continue
// a coding session without losing intent, code context, or user preferences.
func BuildPrompt(msgs []llm.Message, prevSummary string) string {
	var b strings.Builder
	for _, m := range msgs {
		content := strings.TrimSpace(m.Content)
		if content == "" || content == "." || content == clearedMarker {
			continue
		}
		limit := 500
		if m.Role == "tool" {
			limit = 800
		}
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, truncateStr(content, limit))
	}
	conversation := b.String()

	template := NO_TOOLS_PREAMBLE + `
You are a context summarization assistant for coding sessions.
Summarize only the conversation history provided below.
If a previous summary exists, update it incrementally — add new information and remove superseded items.
Do NOT mention that you are summarizing or compacting context.

CRITICAL Preservation Rules:
- Code snippets: preserve FULL code for any file that was modified or is under discussion. Do NOT abbreviate or replace with "updated X". Include the complete function/type definition.
- Error messages: copy VERBATIM — do NOT paraphrase. Exact error text enables accurate future diagnosis.
- File paths: always include the exact path with line numbers when available (e.g., "bot/agent/run.go:212").
- User requirements: use DIRECT QUOTES for constraints or preferences. "User asked for X with Y constraint" is NOT enough — include the actual requirement text.
- Tool output: preserve actual terminal output, grep results, and file contents — these are the ground truth.
- User feedback: if the user corrected something, record exactly what they said and what was wrong.

Output format — use EXACTLY this structure:

<analysis>
1. Scan each message chronologically and identify: user intent, technical decisions, code patterns
2. Extract complete file paths, function signatures, error messages
3. Note any user corrections or feedback
4. Identify what's still pending
(This section will be stripped — use it as your scratchpad to organize before writing the summary.)
</analysis>

<summary>
1. Primary Request and Intent — what the user is trying to accomplish
2. Key Technical Concepts — frameworks, patterns, libraries, constraints
3. Files and Code Sections — exact paths + full code snippets + why each is important
4. Errors and Fixes — verbatim error messages + what fixed them (include user feedback)
5. Problem Solving — approaches tried, what worked, what didn't
6. All User Messages — every substantive user message preserved verbatim (prevents intent drift)
7. Pending Tasks — incomplete work, blocked items, follow-ups needed
8. Current Work — precise description of what was happening when context ran out
9. Optional Next Step — if the next action is clear, state it with a verbatim quote from the original request
</summary>`

	if prevSummary != "" {
		return fmt.Sprintf("%s\n\n[Previous Summary]\n%s\n\n[New Conversation]\n%s\n\n[Updated Structured Summary]:",
			template, prevSummary, conversation)
	}
	return fmt.Sprintf("%s\n\n[Conversation]\n%s\n\n[Structured Summary]:", template, conversation)
}

// FormatCompactSummary strips the <analysis> block and extracts only the
// <summary> content. The analysis block is a scratchpad that helps the model
// organize thoughts — it should not enter the context window.
func FormatCompactSummary(raw string) string {
	// Extract content between <summary> and </summary>.
	start := strings.Index(raw, "<summary>")
	end := strings.Index(raw, "</summary>")
	if start >= 0 && end > start {
		return strings.TrimSpace(raw[start+len("<summary>") : end])
	}
	// Fallback: if no summary tags, strip analysis block manually.
	if idx := strings.Index(raw, "<analysis>"); idx >= 0 {
		if endIdx := strings.Index(raw, "</analysis>"); endIdx > idx {
			raw = raw[:idx] + raw[endIdx+len("</analysis>"):]
		}
	}
	return strings.TrimSpace(raw)
}

// FormatCompactFacts extracts the <key-facts> section from the summarizer output.
// Each non-empty line is treated as one fact. Returns at most 5 facts.
func FormatCompactFacts(raw string) []string {
	start := strings.Index(raw, "<key-facts>")
	end := strings.Index(raw, "</key-facts>")
	if start < 0 || end <= start {
		return nil
	}
	body := raw[start+len("<key-facts>") : end]
	var facts []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "• ")
		line = strings.TrimSpace(line)
		if line == "" || len(line) < 5 {
			continue
		}
		facts = append(facts, line)
	}
	if len(facts) > 5 {
		facts = facts[:5]
	}
	return facts
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
