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
	// Trigger at 80% since micro-compaction handles tool-result bloat first.
	return m.estimatedTokens() > m.tokenBudget*8/10
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
