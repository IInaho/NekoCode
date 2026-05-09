// handlers_done.go — agent 完成后的处理 + token 更新。
package tui

import (
	"fmt"

	"nekocode/tui/components/block"
	tea "charm.land/bubbletea/v2"
	"nekocode/tui/components/message"
)

func (m *Model) handleDone(msg doneMsg) tea.Cmd {
	var finalBlocks []block.ContentBlock
	if msg.err == nil {
		finalBlocks = block.FilterFinalBlocks(m.Messages.ProcessingBlocks())
	}
	m.transitionTo(stateReady)

	if msg.err != nil {
		m.Messages.AddMessage(message.ChatMessage{
			Role:    "error",
			Content: fmt.Sprintf("Error: %v", msg.err),
		})
	} else {
		content := msg.content

		footer := ""
		if msg.duration != "" || msg.tokens != "" {
			footer = "Duration: " + msg.duration
			if msg.tokens != "" {
				footer += "  " + msg.tokens
			}
		}
		m.Messages.AddMessage(message.ChatMessage{
			Role:    "assistant",
			Content: content,
			Footer:  footer,
			Blocks:  finalBlocks,
		})
	}

	prompt, compl := m.Bot.TokenUsage()
	m.Header.SetTokens(prompt + compl)
	m.Messages.GotoBottom()
	return nil
}

