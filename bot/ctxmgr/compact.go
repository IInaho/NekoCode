// compact.go — 微压缩：清理旧的 compactable 工具结果。
package ctxmgr

// compactableTools: 输出量大、可安全清除内容的工具白名单。
// 不包括 task/todo_write — 子 agent 的输出需要保留供后续引用。
var compactableTools = map[string]bool{
	"read": true, "bash": true, "grep": true, "glob": true,
	"web_search": true, "web_fetch": true, "edit": true, "write": true,
}

const keepRecentCompactable = 5
const clearedMarker = "[Old tool result cleared]"

// microCompact content-clears old compactable tool results.
// Keeps the most recent keepRecentCompactable results unmodified.
// Returns the number of results cleared.
// Must be called with the write lock held.
func (m *Manager) microCompact() int {
	var compactable []int
	for i, msg := range m.messages {
		if msg.Role != "tool" || msg.Content == clearedMarker {
			continue
		}
		if m.isCompactableResult(i) {
			compactable = append(compactable, i)
		}
	}
	if len(compactable) <= keepRecentCompactable {
		return 0
	}
	cleared := len(compactable) - keepRecentCompactable
	for _, idx := range compactable[:cleared] {
		m.messages[idx].Content = clearedMarker
	}
	m.compactCount += cleared
	return cleared
}

// isCompactableResult checks if a tool_result at the given index was produced
// by a compactable tool.
func (m *Manager) isCompactableResult(resultIdx int) bool {
	targetID := m.messages[resultIdx].ToolCallID
	if targetID == "" {
		return false
	}
	// Search backward for the assistant message with matching tool call.
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
