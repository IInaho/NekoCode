package agent

import (
	"strings"
	"time"

	"primusbot/command"
)

type InputType int

const (
	InputTypeCommand InputType = iota
	InputTypeText
	InputTypeAgentTask
)

type PerceptionResult struct {
	InputType InputType
	Intent    string
	Entities  map[string]interface{}
	Context   map[string]interface{}
	RawInput  string
}

func isToolIntent(input string) bool {
	patterns := []string{"列出", "查看", "读取", "显示", "找", "搜索", "list", "read", "show", "find", "ls ", "cat ", "grep "}
	lower := strings.ToLower(input)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func PerceiveInput(input string) *PerceptionResult {
	if strings.HasPrefix(input, "/") {
		cmd := command.Parse(input)
		if cmd != nil && cmd.Name != "" {
			return &PerceptionResult{
				InputType: InputTypeCommand,
				Intent:    "execute_command",
				Entities: map[string]interface{}{
					"command": cmd.Name,
					"args":    cmd.Args,
				},
				RawInput: input,
			}
		}
	}

	if strings.HasPrefix(input, "@agent") {
		return &PerceptionResult{
			InputType: InputTypeAgentTask,
			Intent:    "agent_task",
			Entities: map[string]interface{}{
				"task": strings.TrimPrefix(input, "@agent"),
			},
			RawInput: input,
		}
	}

	if isToolIntent(input) {
		return &PerceptionResult{
			InputType: InputTypeAgentTask,
			Intent:    "execute_tool",
			RawInput:  input,
		}
	}

	return &PerceptionResult{
		InputType: InputTypeText,
		Intent:    "chat",
		Context: map[string]interface{}{
			"timestamp":     time.Now(),
			"message_count": 0,
		},
		RawInput: input,
	}
}
