// Model 定义、初始化、listenConfirm goroutine。Model 聚合所有 UI 状态：
// Bot、Header、Messages、Input、Splash、Spinner、Stream、确认、命令提示。
package tui

import (
	"primusbot/bot"
	"primusbot/bot/agent"
	"primusbot/tui/components"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type doneMsg struct {
	content          string
	reasoningContent string
	diffBlocks       string // edit tool diffs, rendered before LLM response
	err              error
}

type confirmMsg struct {
	req agent.ConfirmRequest
}

type Model struct {
	Bot      *bot.Bot
	Header   *components.Header
	Messages *components.Messages
	Input    *components.Input
	Splash   *components.Splash
	Spinner  spinner.Model
	Width    int
	Height   int
	Ready    bool

	Stream      *StreamState

	suggestions       []string
	suggestionIdx     int
	suggestionsVisible bool

	PendingConfirm *agent.ConfirmRequest
	confirmCh      chan agent.ConfirmRequest
}

const Version = "0.1.0"

func NewModel() *Model {
	b := bot.New()
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := &Model{
		Bot:    b,
		Header: components.NewHeader(80, b.Cfg.Provider, b.Cfg.Model, Version),
		Messages: components.NewMessages(80, 14),
		Input:    components.NewInput(80),
		Splash:   components.NewSplash(80, 24, Version),
		Spinner:  sp,
		Stream:   &StreamState{},
		Width:    80,
		Height:   24,
		confirmCh: make(chan agent.ConfirmRequest),
	}

	b.SetConfirmFn(func(req agent.ConfirmRequest) bool {
		m.confirmCh <- req
		return <-req.Response
	})

	used, budget := b.TokenUsage()
	m.Header.SetTokens(used, budget)

	return m
}

func (m *Model) Init() tea.Cmd {
	return m.Input.Init()
}

func listenConfirm(ch <-chan agent.ConfirmRequest) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return confirmMsg{req: req}
	}
}
