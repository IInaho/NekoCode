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
	return m.estimatedTokens() > m.tokenBudget*8/10
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
					newSummary = rs
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

// PostCompactFiles scans messages that were compressed and re-injects the
// most recently read files from the FileStateCache as context attachments.
// This mirrors Claude Code's post-compact file re-creation.
// Returns formatted file content ready for injection as a system message.
func (m *Manager) PostCompactFiles(readFileCache interface{}) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.compactBoundary <= 0 || m.summary == "" {
		return ""
	}

	// Scan compressed messages for Read tool calls.
	type fileRef struct {
		path  string
		index int // higher = more recent
	}
	seen := make(map[string]bool)
	var refs []fileRef

	for i := 0; i < m.compactBoundary && i < len(m.messages); i++ {
		msg := m.messages[i]
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				if tc.Function.Name == "read" {
					// Extract path from arguments (simple scan).
					args := tc.Function.Arguments
					path := extractPathFromArgs(args)
					if path != "" && !seen[path] {
						seen[path] = true
						refs = append(refs, fileRef{path, i})
					}
				}
			}
		}
	}

	// Sort by index descending (most recent first), keep last 5.
	for i := 0; i < len(refs); i++ {
		for j := i + 1; j < len(refs); j++ {
			if refs[j].index > refs[i].index {
				refs[i], refs[j] = refs[j], refs[i]
			}
		}
	}
	if len(refs) > 5 {
		refs = refs[:5]
	}

	if len(refs) == 0 {
		return ""
	}

	// Build the file context attachment.
	var b strings.Builder
	b.WriteString("[Recently read files from compressed context]\n")
	totalBudget := 20000 // 20K chars budget for file re-creation
	for _, ref := range refs {
		if totalBudget <= 0 {
			break
		}
		// Try to get from FileStateCache if available.
		content := getCachedFileContent(readFileCache, ref.path)
		if content == "" {
			continue
		}
		if len(content) > totalBudget {
			content = content[:totalBudget] + "\n..."
		}
		b.WriteString(fmt.Sprintf("\n### %s\n%s\n", ref.path, content))
		totalBudget -= len(content)
	}
	return b.String()
}

// extractPathFromArgs extracts the "path" value from a JSON tool call arguments string.
func extractPathFromArgs(args string) string {
	// Simple extraction: look for "path":"..." or "path": "..."
	idx := strings.Index(args, `"path"`)
	if idx < 0 {
		return ""
	}
	// Find the value after "path":
	rest := args[idx+6:]
	colIdx := strings.Index(rest, ":")
	if colIdx < 0 {
		return ""
	}
	rest = rest[colIdx+1:]
	rest = strings.TrimSpace(rest)
	if len(rest) < 3 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	endIdx := strings.Index(rest, `"`)
	if endIdx < 0 {
		return ""
	}
	return rest[:endIdx]
}

// getCachedFileContent retrieves a file from the cache or reads it fresh.
func getCachedFileContent(cache interface{}, path string) string {
	// Type-assert to FileStateCache interface if available.
	type fileReader interface {
		GetContent(path string) (string, bool)
	}
	if fr, ok := cache.(fileReader); ok {
		if content, found := fr.GetContent(path); found {
			return content
		}
	}
	return ""
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
