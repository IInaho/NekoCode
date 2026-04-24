package agent

import (
	"strings"
)

type ActionResult struct {
	Thought     string
	Action      ActionType
	Output      string
	Error       string
	IsFinal     bool
	ShouldRetry bool
}

func (a *Agent) Execute(reasoning *ReasoningResult) *ActionResult {
	switch reasoning.Action {
	case ActionExecuteTool:
		return a.executeTool(reasoning.ActionInput)
	case ActionChat:
		return &ActionResult{
			Thought: "直接回复用户消息",
			Action:  ActionChat,
			Output:  reasoning.ActionInput,
			IsFinal: true,
		}
	case ActionFinish:
		return &ActionResult{
			Thought: "任务完成",
			Action:  ActionFinish,
			Output:  reasoning.ActionInput,
			IsFinal: true,
		}
	case ActionAskClarification:
		return &ActionResult{
			Thought: "需要更多信息",
			Action:  ActionAskClarification,
			Output:  reasoning.ActionInput,
			IsFinal: false,
		}
	default:
		return &ActionResult{
			Thought: "未知操作类型",
			Action:  ActionChat,
			Output:  "抱歉，我不确定如何处理这个请求",
			IsFinal: true,
		}
	}
}

func (a *Agent) executeTool(input string) *ActionResult {
	if input == "" {
		return &ActionResult{
			Thought:     "没有指定要执行的工具",
			Action:      ActionFinish,
			Error:       "工具调用格式错误",
			ShouldRetry: true,
		}
	}

	parts := strings.SplitN(input, ":", 2)
	if len(parts) != 2 {
		return &ActionResult{
			Thought:     "工具调用格式错误",
			Action:      ActionExecuteTool,
			Error:       "格式应为: 工具名:参数",
			ShouldRetry: true,
		}
	}

	toolName := strings.TrimSpace(parts[0])
	argsStr := strings.TrimSpace(parts[1])

	tool := a.toolRegistry.Get(toolName)
	if tool == nil {
		return &ActionResult{
			Thought:     "工具不存在",
			Action:      ActionExecuteTool,
			Error:       "工具不存在: " + toolName,
			ShouldRetry: true,
		}
	}

	args := parseArguments(argsStr)

	output, err := tool.Execute(nil, args)
	if err != nil {
		return &ActionResult{
			Thought:     "工具执行失败",
			Action:      ActionExecuteTool,
			Error:       err.Error(),
			ShouldRetry: true,
		}
	}

	return &ActionResult{
		Thought: "工具执行成功",
		Action:  ActionExecuteTool,
		Output:  output,
		IsFinal: false,
	}
}

func parseArguments(argsStr string) map[string]interface{} {
	args := make(map[string]interface{})
	if argsStr == "" {
		return args
	}

	pairs := strings.Split(argsStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			args[key] = value
		}
	}

	return args
}
