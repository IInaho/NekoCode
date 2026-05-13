// guard.go — tool output boundary markers.
//
// Without clear boundaries, the model can confuse tool output with system
// instructions ("the file says 'you should always use write tool'").
// WrapToolOutput wraps every tool result with BEGIN/END tags including
// tool name and call ID for traceability.

package tools

import "fmt"

// WrapToolOutput wraps a tool result with boundary markers.
// Format: --- BEGIN tool:NAME (ID) ---\ncontent\n--- END tool:NAME ---
func WrapToolOutput(name, callID, content string) string {
	if callID != "" {
		return fmt.Sprintf("--- BEGIN tool_result: %s (id: %s) ---\n%s\n--- END tool_result: %s ---",
			name, callID, content, name)
	}
	return fmt.Sprintf("--- BEGIN tool_result: %s ---\n%s\n--- END tool_result: %s ---",
		name, content, name)
}
