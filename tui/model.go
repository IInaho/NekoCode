// model.go — Model 结构体 + 初始化 + 状态切换。
package tui

import (
	"fmt"
	"strings"
	"time"

	"nekocode/bot/tools"
	"nekocode/tui/components"
	"nekocode/tui/styles"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type Model struct {
	Bot      BotInterface
	Header   *components.Header
	Messages *components.Messages
	Input    *components.Input
	Splash   *components.Splash
	Spinner  spinner.Model
	Width    int
	Height   int
	Ready    bool

	state           chatState
	processingStart time.Time
	processingPhase string
	Todos           *components.TodoList
	Suggestions     *components.Suggestions
	ConfirmBar      *components.ConfirmBar
	Scrollbar       *components.Scrollbar
	confirmCh       chan tools.ConfirmRequest
}

const version = "0.2.0"

func NewModel(b BotInterface) *Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sty := styles.DefaultStyles()

	m := &Model{
		Bot:         b,
		Header:      components.NewHeader(80, b.Provider(), b.Model(), version),
		Messages:    components.NewMessages(80, 14, &sty),
		Input:       components.NewInput(80),
		Splash:      components.NewSplash(80, 24, version),
		Spinner:     sp,
		Todos:       components.NewTodoList(),
		Suggestions: components.NewSuggestions(&sty),
		ConfirmBar:  components.NewConfirmBar(&sty),
		Scrollbar:   components.NewScrollbar(&sty),
		Width:       80,
		Height:      24,
		state:       stateReady,
		confirmCh:   make(chan tools.ConfirmRequest),
	}

	b.SetConfirmFn(func(req tools.ConfirmRequest) bool {
		m.confirmCh <- req
		return <-req.Response
	})

	b.SetPhaseFn(func(phase string) {
		m.setPhase(phase)
	})

	b.WireTodoWrite(func(items []tools.TodoItem) {
		m.Todos.SetItems(items)
		m.Messages.SetTodos(todoItemsText(items))
		b.SetCtxTodos(todoItemsText(items))
	})

	return m
}

func (m *Model) Init() tea.Cmd {
	return m.Input.Init()
}

func (m *Model) resizeMessages() {
	extra := 0
	if m.state == stateConfirming {
		extra = m.ConfirmBar.Height()
	}
	m.Messages.SetSize(m.Width-1, m.Height-m.Header.Height()-m.Input.Height()-contentMarginV-extra)
}

func (m *Model) transitionTo(state chatState) {
	m.state = state
	switch state {
	case stateReady:
		m.setPhase(PhaseReady)
		m.Messages.SetProcessing(false)
		m.Input.SetSending(false)
		m.ConfirmBar.Clear()
	case stateProcessing:
		m.processingStart = time.Now()
		m.setPhase(PhaseWaiting)
		m.Messages.SetProcessingStatus(PhaseWaiting)

		m.Messages.SetProcessing(true)
		m.Todos.SetItems(nil)
		m.Input.SetSending(true)
	case stateConfirming:
	}
	m.resizeMessages()
}

func listenConfirm(ch <-chan tools.ConfirmRequest) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return confirmMsg{req: req}
	}
}

func todoItemsText(items []tools.TodoItem) string {
	if len(items) == 0 {
		return ""
	}
	done := 0
	for _, it := range items {
		if it.Status == "completed" {
			done++
		}
	}
	if done == len(items) {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Tasks %d/%d\n", done, len(items)))
	for _, it := range items {
		icon := "⬜"
		switch it.Status {
		case "in_progress":
			icon = "🔄"
		case "completed":
			icon = "✅"
		}
		b.WriteString(fmt.Sprintf(" %s %s\n", icon, it.Content))
	}
	return b.String()
}
