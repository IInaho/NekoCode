// 决策模块：每轮通过 callLLMForTool 调用 LLM（Native Function Calling），
// 上下文包含用户输入 + 历史工具调用/结果，LLM 自主决定继续调用工具或回复文本。
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

	toolInput, err := a.callLLMForTool()
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

func (a *Agent) callLLMForTool() (string, error) {
	toolDefs := descriptorsToToolDefs(a.toolRegistry.Descriptors())

	messages := a.ctxMgr.Build(true)

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
