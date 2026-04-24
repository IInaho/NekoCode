package agent

import (
	"fmt"
	"strings"

	"primusbot/llm"
)

type ActionType int

const (
	ActionChat ActionType = iota
	ActionExecuteTool
	ActionFinish
	ActionAskClarification
)

type ReasoningResult struct {
	Thought        string
	Action         ActionType
	ActionInput    string
	ShouldContinue bool
	IsFinal        bool
}

type ReasoningContext struct {
	InputType     InputType
	Intent        string
	History       string
	Tools         string
	CurrentStep   int
	MaxIterations int
}

func (a *Agent) Reason(state *PerceptionResult) *ReasoningResult {
	_ = a.buildReasoningContext(state)

	if state.InputType == InputTypeCommand {
		return &ReasoningResult{
			Thought:        "用户输入了命令，直接执行命令",
			Action:         ActionFinish,
			ActionInput:    "",
			ShouldContinue: false,
			IsFinal:        true,
		}
	}

	if state.Intent == "chat" {
		return &ReasoningResult{
			Thought:        "用户想要聊天，直接回复",
			Action:         ActionChat,
			ActionInput:    state.RawInput,
			ShouldContinue: false,
			IsFinal:        true,
		}
	}

	if state.Intent == "execute_tool" || state.Intent == "observation" || state.Intent == "agent_task" {
		if state.Intent == "agent_task" {
			task, _ := state.Entities["task"].(string)
			if task != "" {
				state.RawInput = task
			}
		}

		previousAction, _ := state.Entities["previous_action"].(string)
		previousOutput, _ := state.Entities["previous_output"].(string)
		success, _ := state.Entities["success"].(bool)

		if previousAction == "execute_tool" && success {
			toolResponse, err := a.callLLMForResponse(state.RawInput, previousOutput)
			if err != nil {
				return &ReasoningResult{
					Thought:        "生成回复失败",
					Action:         ActionChat,
					ActionInput:    previousOutput,
					ShouldContinue: false,
					IsFinal:        true,
				}
			}
			return &ReasoningResult{
				Thought:        "工具执行成功，生成回复",
				Action:         ActionChat,
				ActionInput:    toolResponse,
				ShouldContinue: false,
				IsFinal:        true,
			}
		}

		toolInput, err := a.callLLMForTool(state.RawInput)
		if err != nil {
			return &ReasoningResult{
				Thought:        "LLM调用失败",
				Action:         ActionChat,
				ActionInput:    fmt.Sprintf("抱歉，调用工具失败: %v", err),
				ShouldContinue: false,
				IsFinal:        true,
			}
		}
		if toolInput == "" {
			return &ReasoningResult{
				Thought:        "无法确定要使用的工具",
				Action:         ActionChat,
				ActionInput:    "抱歉，我无法确定要使用哪个工具来完成任务",
				ShouldContinue: false,
				IsFinal:        true,
			}
		}
		return &ReasoningResult{
			Thought:        "调用工具: " + toolInput,
			Action:         ActionExecuteTool,
			ActionInput:    toolInput,
			ShouldContinue: true,
			IsFinal:        false,
		}
	}

	return &ReasoningResult{
		Thought:        "需要使用Agent模式处理",
		Action:         ActionExecuteTool,
		ActionInput:    "",
		ShouldContinue: true,
		IsFinal:        false,
	}
}

func (a *Agent) buildReasoningContext(state *PerceptionResult) *ReasoningContext {
	history := a.memory.Summary()
	tools := a.toolRegistry.AvailableToolsString()

	return &ReasoningContext{
		InputType:     state.InputType,
		Intent:        state.Intent,
		History:       history,
		Tools:         tools,
		CurrentStep:   a.currentStep,
		MaxIterations: a.maxIterations,
	}
}

func (a *Agent) buildReasoningPrompt(context *ReasoningContext) string {
	return fmt.Sprintf(`
当前状态：
- 输入类型：%d
- 意图：%s
- 当前步骤：%d / %d

可用工具：
%s

历史记录：
%s

请决定下一步行动。返回格式：
Thought: <你的思考>
Action: <execute_tool|chat|finish>
ActionInput: <工具名:参数 或 消息内容>
`, context.InputType, context.Intent,
		context.CurrentStep, context.MaxIterations,
		context.Tools, context.History)
}

func (a ActionType) String() string {
	switch a {
	case ActionChat:
		return "chat"
	case ActionExecuteTool:
		return "execute_tool"
	case ActionFinish:
		return "finish"
	case ActionAskClarification:
		return "ask_clarification"
	default:
		return "unknown"
	}
}

func parseActionType(s string) ActionType {
	switch s {
	case "execute_tool":
		return ActionExecuteTool
	case "chat":
		return ActionChat
	case "finish":
		return ActionFinish
	case "ask_clarification":
		return ActionAskClarification
	default:
		return ActionChat
	}
}

func (a *Agent) callLLMForTool(input string) (string, error) {
	tools := a.toolRegistry.AvailableToolsString()

	prompt := fmt.Sprintf(`你是一个AI助手，需要根据用户请求决定使用哪个工具。

用户请求: %s

可用工具:
%s

请根据用户请求选择一个合适的工具并生成调用参数。

示例：
- 用户说"列出当前目录" → 返回: bash:command=ls
- 用户说"查看main.go内容" → 返回: bash:command=cat main.go
- 用户说"搜索go文件" → 返回: glob:pattern=*.go
- 用户说"读取文件README.md" → 返回: filesystem:operation=read,path=README.md

请直接返回工具调用格式（不要有任何解释或markdown格式）。`, input, tools)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := a.llmClient.Chat(a.ctx, messages)
	if err != nil {
		return "", err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	content = strings.TrimSpace(content)
	content = strings.Trim(content, "\"")
	return content, nil
}

func (a *Agent) callLLMForResponse(userInput, toolOutput string) (string, error) {
	prompt := fmt.Sprintf(`用户请求: %s

工具执行结果:
%s

请根据工具执行结果，用友好的方式回复用户。只返回回复内容，不要返回其他内容。`, userInput, toolOutput)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	resp, err := a.llmClient.Chat(a.ctx, messages)
	if err != nil {
		return "", err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return strings.TrimSpace(content), nil
}
