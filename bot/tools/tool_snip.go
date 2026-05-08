// tool_snip.go — Snip tool: lets the model proactively remove old messages.
package tools

import (
	"context"
	"fmt"
)

// SnipFunc is called when the model invokes snip to remove a message range.
type SnipFunc func(startIdx, endIdx int) string

type SnipTool struct {
	snipFn SnipFunc
}

func NewSnipTool() *SnipTool { return &SnipTool{} }

func (t *SnipTool) Wire(fn SnipFunc) { t.snipFn = fn }

func (t *SnipTool) Name() string                                       { return "snip" }
func (t *SnipTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeSequential }
func (t *SnipTool) DangerLevel(map[string]interface{}) DangerLevel     { return LevelDestructive }

func (t *SnipTool) Description() string {
	return "Remove old messages no longer needed to save context. Only use for completed conversation segments with no further reference value. Irreversible."
}

func (t *SnipTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "start_index", Type: "number", Required: true, Description: "First message index to remove (inclusive)"},
		{Name: "end_index", Type: "number", Required: true, Description: "Last message index to remove (inclusive)"},
	}
}

func (t *SnipTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.snipFn == nil {
		return "", fmt.Errorf("snip tool: not wired")
	}
	start, ok := args["start_index"].(float64)
	if !ok {
		return "", fmt.Errorf("missing start_index parameter")
	}
	end, ok := args["end_index"].(float64)
	if !ok {
		return "", fmt.Errorf("missing end_index parameter")
	}
	return t.snipFn(int(start), int(end)), nil
}
