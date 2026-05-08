// agent.go — 启动 agent 对话流程：startChat、startAgent、工具函数。
package tui

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"primusbot/tui/components/block"
	"primusbot/tui/components/message"
	"primusbot/tui/styles"

	tea "charm.land/bubbletea/v2"
)

func logPanic(r any) {
	stack := debug.Stack()
	path := fmt.Sprintf("primusbot-panic-%d.log", time.Now().Unix())
	msg := fmt.Sprintf("PANIC: %v\n\nStack:\n%s", r, string(stack))
	_ = os.WriteFile(path, []byte(msg), 0644)
}

func (m *Model) startChat(value string) tea.Cmd {
	resp, ok := m.Bot.ExecuteCommand(value)
	if ok && resp != "" {
		m.Messages.AddMessage(message.ChatMessage{Role: "system", Content: resp})
		return nil
	}
	return m.startAgent(value)
}

func (m *Model) startAgent(value string) tea.Cmd {
	m.Messages.AddMessage(message.ChatMessage{Role: "user", Content: value})
	m.Messages.GotoBottom()
	m.Input.SetFollow(true)
	m.transitionTo(stateProcessing)

	m.Bot.SetStreamFn(func(delta string) { m.Messages.ProcessStreamText(delta) })
	m.Bot.SetReasoningStreamFn(func(delta string) { m.Messages.ProcessReasoningText(delta) })

	return tea.Batch(
		spinnerTick(),
		listenConfirm(m.confirmCh),
		m.runAgent(value),
	)
}

func (m *Model) runAgent(value string) func() tea.Msg {
	return func() tea.Msg {
		defer func() {
			if r := recover(); r != nil {
				logPanic(r)
			}
		}()

		var finalResponse string

		result, err := m.Bot.RunAgent(value, m.onAgentStep(&finalResponse))

		if finalResponse == "" {
			finalResponse = result
		}
		if finalResponse == "" {
			finalResponse = "sorry, could not complete this task."
		}

		return doneMsg{
			content:  finalResponse,
			duration: m.Bot.Duration(),
			tokens:   tokensSummary(m.Bot),
			err:      err,
		}
	}
}

func (m *Model) onAgentStep(finalResponse *string) func(int, string, string, string, string, string, int, int) {
	return func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int) {
		switch {
		case action == "think":
		case action == "chat":
			*finalResponse = output
			m.Messages.AddThinkBlock(output)
		case action == "tool_start":
			m.Messages.ProcessToolBlock(block.ContentBlock{
				Type:       block.BlockTool,
				ToolName:   toolName,
				ToolArgs:   formatBriefArgs(toolName, toolArgs),
				Collapsed:  true,
				BatchIdx:   batchIdx,
				BatchTotal: batchTotal,
			})
		case toolName != "":
			if toolName == "edit" && output != "" {
				m.Messages.AddDiffBlock(extractDiffContent(output))
			}
		}
	}
}

// extractDiffContent strips the edit tool's header line, leaving only -/+/context lines.
func extractDiffContent(output string) string {
	idx := strings.Index(output, "\n")
	if idx < 0 {
		return output
	}
	return output[idx+1:]
}

func tokensSummary(b BotInterface) string {
	p, c := b.TokenUsage()
	return "↑" + styles.FmtTokens(p) + " ↓" + styles.FmtTokens(c)
}
