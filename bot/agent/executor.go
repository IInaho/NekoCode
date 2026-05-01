// 动作执行：根据 ReasoningResult 调度具体行为。
// ActionChat → 直接返回文本；ActionExecuteTool → 解析工具调用、
// 检查 DangerLevel、必要时通过 confirmFn 征求确认、执行工具。
package agent

import (
	"fmt"

	"primusbot/bot/tools"
	"primusbot/bot/types"
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
	toolName, args, err := tools.ParseCall(input)
	if err != nil {
		return &ActionResult{
			Thought:     "工具调用格式错误",
			Action:      ActionExecuteTool,
			Error:       err.Error(),
			ShouldRetry: true,
		}
	}

	tool, err := a.toolRegistry.Get(toolName)
	if err != nil {
		return &ActionResult{
			Thought:     "工具不存在",
			Action:      ActionExecuteTool,
			Error:       err.Error(),
			ShouldRetry: true,
		}
	}

	level := tool.DangerLevel(args)
	if level == tools.LevelForbidden {
		return &ActionResult{
			Thought:     "禁止执行危险操作",
			Action:      ActionExecuteTool,
			Error:       fmt.Sprintf("操作被拒绝: %s 属于禁止操作", toolName),
			ShouldRetry: false,
		}
	}

	if level >= tools.LevelWrite && a.confirmFn != nil {
		if !a.confirmFn(types.ConfirmRequest{
			ToolName: toolName,
			Args:     args,
			Level:    level,
			Response: make(chan bool, 1),
		}) {
			return &ActionResult{
				Thought:     "用户取消了操作",
				Action:      ActionExecuteTool,
				Error:       "操作被用户取消",
				ShouldRetry: false,
			}
		}
	}

	if a.phaseFn != nil {
		a.phaseFn("Running " + toolName)
		}
		output, err := tool.Execute(a.ctx, args)
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
