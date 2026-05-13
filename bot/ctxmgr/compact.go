// compact.go — tiered micro-compaction for tool results.
//
// Tool results have different reuse value. File content (read, edit, write) is
// referenced across turns; search results (grep, glob, list) are one-shot
// navigation aids. Compaction now clears low-value results first.
package ctxmgr

import "sort"

// compactableTools: output-heavy tools whose results can be safely cleared.
// task/todo_write are excluded — sub-agent output and task state must persist.
var compactableTools = map[string]bool{
	"read": true, "bash": true, "grep": true, "glob": true,
	"web_search": true, "web_fetch": true, "edit": true, "write": true,
}

// Priority tiers for tool result retention during compaction.
const (
	priorityLow    = iota // clear first: one-shot navigation
	priorityMedium        // clear second: valuable but time-sensitive
	priorityHigh          // clear last: file content referenced across turns
)

const clearedMarker = "[Old tool result cleared]"

// compactableToolPriority returns the retention priority for a tool result.
// Higher = keep longer.
func compactableToolPriority(toolName, content string) int {
	switch toolName {
	case "read", "edit", "write":
		return priorityHigh
	case "bash":
		// Distinguish: build errors / test output (valuable) vs trivial commands.
		// Commands whose output is just an exit code or a few chars are low-value.
		if len(content) > 120 {
			return priorityMedium
		}
		return priorityLow
	case "web_search", "web_fetch":
		return priorityMedium
	case "grep", "glob", "list":
		return priorityLow
	default:
		return priorityLow
	}
}

// lookupToolName finds the tool name that produced the tool result at the given
// index by walking backward to the matching assistant message.
func (m *Manager) lookupToolName(resultIdx int) string {
	targetID := m.messages[resultIdx].ToolCallID
	if targetID == "" {
		return ""
	}
	for i := resultIdx - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			for _, tc := range m.messages[i].ToolCalls {
				if tc.ID == targetID {
					return tc.Function.Name
				}
			}
		}
	}
	return ""
}

type compactable struct {
	idx      int
	priority int
}

// lookupAssistantIdx finds the assistant message that owns the tool result at
// resultIdx by walking backward to the nearest matching assistant message.
func (m *Manager) lookupAssistantIdx(resultIdx int) int {
	targetID := m.messages[resultIdx].ToolCallID
	if targetID == "" {
		return -1
	}
	for i := resultIdx - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			for _, tc := range m.messages[i].ToolCalls {
				if tc.ID == targetID {
					return i
				}
			}
		}
	}
	return -1
}

// microCompact content-clears old compactable tool results, but preserves
// results from the last 2 turns — the LLM is actively working with those.
// Among older results, clears low-priority groups first. Clearing is batch-
// atomic: all results from the same assistant message stay or go together,
// preventing filterValidMessages from dropping entire batches due to
// partial clearing.
// Must be called with the write lock held.
func (m *Manager) microCompact() int {
	recentBoundary := m.findRecentTurnBoundary(2)
	if recentBoundary < 0 {
		recentBoundary = 0
	}

	// Collect compactable results, grouped by their owning assistant message.
	type batch struct {
		assistantIdx int
		results      []compactable
	}
	batches := make(map[int]*batch)
	for i, msg := range m.messages {
		if msg.Role != "tool" || msg.Content == clearedMarker {
			continue
		}
		if !m.isCompactableResult(i) {
			continue
		}
		if i >= recentBoundary {
			continue
		}
		assistantIdx := m.lookupAssistantIdx(i)
		if assistantIdx < 0 {
			continue
		}
		toolName := m.lookupToolName(i)
		pri := compactableToolPriority(toolName, msg.Content)
		b := batches[assistantIdx]
		if b == nil {
			b = &batch{assistantIdx: assistantIdx}
			batches[assistantIdx] = b
		}
		b.results = append(b.results, compactable{idx: i, priority: pri})
	}

	// Each batch inherits the HIGHEST priority of its members — if any result
	// is valuable, the whole batch is valuable. This prevents a low-value
	// result from being cleared while its high-value siblings remain, which
	// would cause filterValidMessages to drop the entire assistant message.
	var batchList []*batch
	for _, b := range batches {
		maxPri := priorityLow
		for _, r := range b.results {
			if r.priority > maxPri {
				maxPri = r.priority
			}
		}
		for i := range b.results {
			b.results[i].priority = maxPri
		}
		batchList = append(batchList, b)
	}

	sort.Slice(batchList, func(a, b int) bool {
		if batchList[a].results[0].priority != batchList[b].results[0].priority {
			return batchList[a].results[0].priority < batchList[b].results[0].priority
		}
		return batchList[a].assistantIdx < batchList[b].assistantIdx
	})

	keepResults := 3
	switch {
	case m.tokenBudget >= 128000:
		keepResults = 8
	case m.tokenBudget >= 64000:
		keepResults = 5
	}

	// Count total compactable results across all batches.
	total := 0
	for _, b := range batchList {
		total += len(b.results)
	}
	if total <= keepResults {
		return 0
	}

	// Clear entire batches from the front (lowest priority first), but
	// stop before the remaining result count drops below keepResults.
	cleared := 0
	kept := total
	for _, b := range batchList {
		if kept - len(b.results) < keepResults {
			break
		}
		for _, r := range b.results {
			m.messages[r.idx].Content = clearedMarker
			cleared++
		}
		kept -= len(b.results)
	}
	m.compactCount += cleared
	return cleared
}

// findRecentTurnBoundary returns the message index where the Nth-to-last user
// turn begins. Results at or after this index are considered "recent".
func (m *Manager) findRecentTurnBoundary(n int) int {
	userCount := 0
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			userCount++
			if userCount >= n {
				return i
			}
		}
	}
	return 0 // fewer than n turns total, protect everything
}

// isCompactableResult checks if a tool_result at the given index was produced
// by a compactable tool.
func (m *Manager) isCompactableResult(resultIdx int) bool {
	targetID := m.messages[resultIdx].ToolCallID
	if targetID == "" {
		return false
	}
	for i := resultIdx - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			for _, tc := range m.messages[i].ToolCalls {
				if tc.ID == targetID {
					return compactableTools[tc.Function.Name]
				}
			}
		}
	}
	return false
}
