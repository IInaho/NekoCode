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
	run  SubAgentFunc
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

func (t *TaskTool) Name() string                                   { return "task" }
func (t *TaskTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
func (t *TaskTool) DangerLevel(map[string]interface{}) DangerLevel    { return LevelSafe }
func (t *TaskTool) Description() string {
	return `重型工具：将复杂子任务委派给专用子 agent。简单文件写入请直接用 write——不要为每个操作 spawn subagent。

仅在以下场景使用：
- executor：大型重构、多文件复杂改动（需要独立上下文和3+步推理）
- explore：大规模代码库探索（需要3+轮搜索时）
- verify：项目有测试套件且需要对抗性验证（边界值/并发/异常输入）。简单项目的 go build 请自己跑
- plan：需要用户审批方案的场景
- decompose：将大型方案拆解为并行子任务

subagent 看不到对话历史，prompt 必须自包含。多个独立 task 一次发送即可并行执行。`
}

func (t *TaskTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "description", Type: "string", Required: true, Description: "任务简述（必填，≤40字），如: 创建 types.go 类型定义"},
		{Name: "prompt", Type: "string", Required: true, Description: "任务描述。必须自包含：子 agent 不知道你的对话历史、工具输出、文件内容。包含所有文件路径、代码上下文、错误信息、约束和期望输出格式。"},
		{Name: "subagent_type", Type: "string", Required: false, Description: "子 agent 类型：plan / decompose / executor / verify / explore。默认 executor"},
	}
}

func (t *TaskTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	if t.run == nil {
		return "", fmt.Errorf("task tool: not wired")
	}

	prompt, ok := args["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("缺少 prompt 参数")
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
