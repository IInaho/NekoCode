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
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.summarizer == nil && !useSessionMemory {
		return nil
	}

	keep := m.windowSize / 2
	if keep < 2 {
		keep = 2
	}

	// Tail preservation: ensure the last N user turns are never split or
	// compressed. This prevents recency bias from causing goal drift in
	// long conversations. Scan backward from the end to find the boundary
	// of the last 3 user messages, and extend keep to include them.
	const preserveTurns = 3
	tailKeep := m.countMessagesForLastNTurns(preserveTurns)
	if tailKeep > keep {
		keep = tailKeep
	}

	if len(m.messages) <= keep {
		return nil
	}

	split := len(m.messages) - keep

	// Only summarize messages after the previous compact boundary.
	// Messages before compactBoundary were already summarized — don't
	// re-send them to the LLM unnecessarily.
	start := m.compactBoundary
	if split <= start {
		// Nothing new to compress beyond the existing boundary.
		return nil
	}

	if useSessionMemory {
		// Free summary from session memory.
		m.summary = smContent
	} else {
		toSummarize := make([]llm.Message, split-start)
		copy(toSummarize, m.messages[start:split])

		newSummary, err := m.summarizer(toSummarize, m.summary)
		if err != nil {
			return fmt.Errorf("summarize: %w", err)
		}
		m.summary = newSummary

			// Verify critical constraints survived compression.
			if missing := m.verifySummary(newSummary); len(missing) > 0 {
				verifyPrompt := BuildVerifyPrompt(toSummarize, m.summary, missing)
				if rs, re := m.summarizer([]llm.Message{{Role: "user", Content: verifyPrompt}}, m.summary); re == nil && rs != "" {
					m.summary = rs
				}
			}
	}

	// Move the compact boundary forward to this split point.
	// Messages before the boundary are preserved (not dropped) for:
	//   - session memory extraction (needs full history)
	//   - stable [id:N] indices for the snip tool
	//   - post-compact file re-creation
	// Build() only sends messages after compactBoundary.
	m.compactBoundary = split

	// Trim very old messages (before compactBoundary) to prevent unbounded growth.
	// Keep at most 200 messages before the boundary for memory extraction context.
	const maxPreservedBeforeBoundary = 200
	if m.compactBoundary > maxPreservedBeforeBoundary {
		trim := m.compactBoundary - maxPreservedBeforeBoundary
		m.messages = m.messages[trim:]
		m.compactBoundary -= trim
		// Rebuild snipped map with adjusted indices so snip still targets
		// the correct messages after the shift.
		oldSnipped := m.snipped
		m.snipped = make(map[int]bool, len(oldSnipped))
		for idx := range oldSnipped {
			if idx >= trim {
				m.snipped[idx-trim] = true
			}
			// idx < trim → message was trimmed away, drop it.
		}
	}

	return nil
}

// countMessagesForLastNTurns counts the number of messages belonging to the
// last n user turns. Scans backward from the end, counting a "turn" from the
// user message through all subsequent assistant/tool messages.
// Must be called with the write lock held.
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

// BuildPrompt assembles a structured summarization prompt from messages.
func BuildPrompt(msgs []llm.Message, prevSummary string) string {
	var b strings.Builder
	for _, m := range msgs {
		// Skip empty content, placeholder dots, and cleared markers
		// so they don't pollute the summary with noise.
		content := strings.TrimSpace(m.Content)
		if content == "" || content == "." || content == clearedMarker {
			continue
		}
		limit := 500
		if m.Role == "tool" {
			limit = 800 // tool results carry more signal
		}
		fmt.Fprintf(&b, "[%s]: %s\n", m.Role, truncateStr(content, limit))
	}
	conversation := b.String()

	template := `You are an anchored context summarization assistant for coding sessions.
Summarize only the conversation history provided below.
If a previous summary exists, update it incrementally — add new information and remove superseded items.
Do NOT mention that you are summarizing or compacting context.
Keep each section concise.

CRITICAL Preservation Rules:
- Code snippets: preserve FULL code for any file that was modified or is under discussion. Do NOT abbreviate or replace with "updated X".
- Error messages: copy verbatim — do NOT paraphrase. Error text must be exact so future diagnosis is possible.
- File paths: always include the exact path with line numbers when available (e.g., "bot/agent/run.go:212").
- User requirements: use direct quotes for constraints or preferences the user specified. "User asked for X with Y constraint" is NOT enough — include the actual requirement text.

Output in this exact format:

[Goal]
What is the user trying to accomplish (1-2 sentences)

[Progress]
Done: steps that have been resolved or completed
In Progress: work currently being done
Blocked: current blockers or obstacles

[Key Decisions]
Important technical choices, architecture decisions, or trade-offs

[Next Steps]
What actions should be taken next

[Critical Context]
User preferences, constraints, environment info that must be remembered

[Relevant Files]
Key file paths ordered by importance, with notes on their role`

	if prevSummary != "" {
		return fmt.Sprintf("%s\n\n[Previous Summary]\n%s\n\n[New Conversation]\n%s\n\n[Updated Structured Summary]:",
			template, prevSummary, conversation)
	}
	return fmt.Sprintf("%s\n\n[Conversation]\n%s\n\n[Structured Summary]:", template, conversation)
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
