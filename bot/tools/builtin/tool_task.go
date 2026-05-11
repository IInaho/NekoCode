package builtin

import (
	"context"
	"fmt"
	"strings"
	"nekocode/bot/tools"
)

// SubAgentFunc is the function signature for running a sub-agent.
// The tools package doesn't import subagent — it receives this at wire time.
type SubAgentFunc func(ctx context.Context, prompt, agentType string) (string, error)

type TaskTool struct {
	run   SubAgentFunc
	names []string
}

func NewTaskTool() *TaskTool {
	return &TaskTool{}
}

// Wire sets the sub-agent runner and available agent type names.
func (t *TaskTool) Wire(run SubAgentFunc, names []string) {
	t.run = run
	t.names = names
}

func (t *TaskTool) Name() string                                       { return "task" }
func (t *TaskTool) ExecutionMode(map[string]interface{}) tools.ExecutionMode { return tools.ModeParallel }
func (t *TaskTool) DangerLevel(map[string]interface{}) tools.DangerLevel     { return tools.LevelSafe }
func (t *TaskTool) Description() string {
	return `HEAVY/EXPENSIVE — DO NOT use for simple tasks you can do yourself. Read, edit, write, bash are much faster.

ONLY use when ALL conditions are true:
- executor: 5+ files across multiple packages, AND too complex for one turn
- explore: 3+ search rounds needed across the codebase
- verify: project has tests AND you made non-trivial changes (NOT simple build check)
- plan/decompose: user explicitly asked for a plan

Subagents CANNOT see conversation history — prompts must include ALL context (file paths, errors, code snippets). If your prompt is under 200 chars, the task is too simple for a subagent.`
}

func (t *TaskTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{Name: "description", Type: "string", Required: true, Description: "Task summary (required, ≤40 chars), e.g. 'create types.go type definitions'"},
		{Name: "prompt", Type: "string", Required: true, Description: "Task description. Must be self-contained: the sub-agent cannot see your conversation history, tool outputs, or file contents. Include all file paths, code context, error messages, constraints, and expected output format."},
		{Name: "subagent_type", Type: "string", Required: false, Description: "Sub-agent type: plan / decompose / executor / verify / explore. Default: executor"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.run == nil {
		return "", fmt.Errorf("task tool: not wired")
	}

	prompt, ok := args["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("missing prompt parameter")
	}

	// Enforce description: auto-generate if missing, truncate if too long.
	desc, _ := args["description"].(string)
	if desc == "" {
		desc = strings.SplitN(prompt, "\n", 2)[0]
		desc = strings.Trim(desc, " \"")
	}
	desc = tools.TruncateByRune(desc, 40)
	args["description"] = desc

	typeName := "executor"
	if s, ok := args["subagent_type"].(string); ok && s != "" {
		typeName = s
	}

	return t.run(ctx, prompt, typeName)
}
