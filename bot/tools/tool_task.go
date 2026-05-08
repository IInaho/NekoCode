package tools

import (
	"context"
	"fmt"
	"strings"
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
func (t *TaskTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
func (t *TaskTool) DangerLevel(map[string]interface{}) DangerLevel     { return LevelSafe }
func (t *TaskTool) Description() string {
	return `Heavy tool: delegate complex sub-tasks to specialized sub-agents. For simple file writes, use write directly — don't spawn a subagent for every operation.

Use only in these scenarios:
- executor: large refactors, complex multi-file changes (needs independent context and 3+ reasoning steps)
- explore: large-scale codebase exploration (needs 3+ search rounds)
- verify: project has tests and needs adversarial validation (edge cases/concurrency/malformed input). For simple go build, do it yourself
- plan: when user needs to approve the approach
- decompose: break large plans into parallel sub-tasks

Subagents can't see conversation history — prompts must be self-contained. Multiple independent tasks can be sent at once for parallel execution.`
}

func (t *TaskTool) Parameters() []Parameter {
	return []Parameter{
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
	desc = TruncateByRune(desc, 40)
	args["description"] = desc

	typeName := "executor"
	if s, ok := args["subagent_type"].(string); ok && s != "" {
		typeName = s
	}

	return t.run(ctx, prompt, typeName)
}
