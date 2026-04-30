// 决策模块：根据当前 stepState 决定下一步动作。
// - 斜杠命令 → ActionFinish
// - 上一轮工具成功 → callLLMForResponse 生成最终回复
// - 其他情况 → callLLMForTool（Native Function Calling）让 LLM 选择工具或直接聊天
// descriptorsToToolDefs 将 tools.Descriptor 转为 llm.ToolDef 供 API 调用。
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"primusbot/bot/tools"
	"primusbot/llm"
)

type ActionType int

const (
	ActionChat ActionType = iota
	ActionExecuteTool
	ActionFinish
)

type ReasoningResult struct {
	Thought     string
	Action      ActionType
	ActionInput string
	IsFinal     bool
}

func (a *Agent) Reason(state *stepState) *ReasoningResult {
	if strings.HasPrefix(state.input, "/") {
		return &ReasoningResult{
			Thought:     "用户输入了命令",
			Action:      ActionFinish,
			ActionInput: "",
			IsFinal:     true,
		}
	}

	if state.previousAction == "execute_tool" && state.success {
		response, err := a.callLLMForResponse(state.input, state.previousOutput)
		if err != nil {
			return &ReasoningResult{
				Thought:     "生成回复失败",
				Action:      ActionChat,
				ActionInput: state.previousOutput,
				IsFinal:     true,
			}
		}
		return &ReasoningResult{
			Thought:     "工具执行成功，生成回复",
			Action:      ActionChat,
			ActionInput: response,
			IsFinal:     true,
		}
	}

	toolInput, err := a.callLLMForTool(state.input)
	if err != nil {
		return &ReasoningResult{
			Thought:     "LLM调用失败",
			Action:      ActionChat,
			ActionInput: fmt.Sprintf("抱歉，调用失败: %v", err),
			IsFinal:     true,
		}
	}

	if toolInput == "" {
		return &ReasoningResult{
			Thought:     "无法确定要使用的工具",
			Action:      ActionChat,
			ActionInput: "抱歉，我无法确定要做什么",
			IsFinal:     true,
		}
	}

	if strings.Contains(toolInput, ":") {
		return &ReasoningResult{
			Thought:     "调用工具: " + toolInput,
			Action:      ActionExecuteTool,
			ActionInput: toolInput,
			IsFinal:     false,
		}
	}

	return &ReasoningResult{
		Thought:     "直接回复",
		Action:      ActionChat,
		ActionInput: toolInput,
		IsFinal:     true,
	}
}

func (a ActionType) String() string {
	switch a {
	case ActionChat:
		return "chat"
	case ActionExecuteTool:
		return "execute_tool"
	case ActionFinish:
		return "finish"
	default:
		return "unknown"
	}
}

func (a *Agent) callLLMForTool(input string) (string, error) {
	toolDefs := descriptorsToToolDefs(a.toolRegistry.Descriptors())

	messages := a.ctxMgr.Build(true)
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: input,
	})

	resp, err := a.llmClient.Chat(a.ctx, messages, toolDefs)
	if err != nil {
		return "", err
	}

	toolCalls := llm.LastToolCalls(resp)
	if len(toolCalls) == 0 {
		if len(resp.Choices) > 0 {
			return resp.Choices[0].Message.Content, nil
		}
		return "", nil
	}

	tc := toolCalls[0]
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("解析工具参数失败: %v", err)
	}
	return tc.Function.Name + ":" + formatArgs(args), nil
}

func (a *Agent) callLLMForResponse(userInput, toolOutput string) (string, error) {
	messages := a.ctxMgr.Build(false)
	messages = append(messages, llm.Message{
		Role:    "user",
		Content: fmt.Sprintf("用户请求: %s\n\n工具执行结果:\n%s\n\n请根据工具执行结果，用友好的方式回复用户。", userInput, toolOutput),
	})

	resp, err := a.llmClient.Chat(a.ctx, messages, nil)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) > 0 {
		return strings.TrimSpace(resp.Choices[0].Message.Content), nil
	}
	return "", nil
}

func descriptorsToToolDefs(descs []tools.Descriptor) []llm.ToolDef {
	defs := make([]llm.ToolDef, len(descs))
	for i, d := range descs {
		props := make(map[string]llm.Property)
		var required []string
		for _, p := range d.Parameters {
			props[p.Name] = llm.Property{
				Type:        p.Type,
				Description: p.Description,
			}
			if p.Required {
				required = append(required, p.Name)
			}
		}
		defs[i] = llm.ToolDef{
			Type: "function",
			Function: llm.FunctionDef{
				Name:        d.Name,
				Description: d.Description,
				Parameters: llm.Parameters{
					Type:       "object",
					Properties: props,
					Required:   required,
				},
			},
		}
	}
	return defs
}
