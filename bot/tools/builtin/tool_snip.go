// tool_snip.go — Snip tool: lets the model proactively remove old messages.
package builtin

import (
	"context"
	"fmt"
	"nekocode/bot/tools"
)

type SnipTool struct {
	snipFn tools.SnipFunc
}

func NewSnipTool() *SnipTool { return &SnipTool{} }

func (t *SnipTool) Wire(fn tools.SnipFunc) { t.snipFn = fn }

func (t *SnipTool) Name() string                                       { return "snip" }
func (t *SnipTool) ExecutionMode(map[string]interface{}) tools.ExecutionMode { return tools.ModeSequential }
func (t *SnipTool) DangerLevel(map[string]interface{}) tools.DangerLevel     { return tools.LevelDestructive }

func (t *SnipTool) Description() string {
	return "Remove old messages no longer needed to save context. Only use for completed conversation segments with no further reference value. Irreversible."
}

func (t *SnipTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
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
