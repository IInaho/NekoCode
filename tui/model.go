package tui

import (
	"primusbot/bot/types"
	"primusbot/tui/components"
	"primusbot/tui/styles"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// BotInterface is the contract any bot implementation must satisfy for the TUI.
type BotInterface interface {
	RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string)) (string, error)
	ExecuteCommand(input string) (string, bool)
	TokenUsage() (int, int)
	CommandNames() []string
	SetConfirmFn(types.ConfirmFunc)
	SetPhaseFn(types.PhaseFunc)
	Provider() string
	Model() string
}

type ChatState int

const (
	StateReady ChatState = iota
	StateProcessing
	StateConfirming
)

type doneMsg struct {
	content    string
	diffBlocks string
	err        error
}

type confirmMsg struct {
	req types.ConfirmRequest
}

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

	Stream          *BlockStream
	state           ChatState
	processingStart time.Time
	processingPhase string
	Suggestions     *components.Suggestions
	ConfirmBar      *components.ConfirmBar

	confirmCh chan types.ConfirmRequest
}

const Version = "0.1.0"

func NewModel(b BotInterface) *Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sty := styles.DefaultStyles()

	m := &Model{
		Bot:         b,
		Header:      components.NewHeader(80, b.Provider(), b.Model(), Version),
		Messages:    components.NewMessages(80, 14, &sty),
		Input:       components.NewInput(80),
		Splash:      components.NewSplash(80, 24, Version),
		Spinner:     sp,
		Stream:      &BlockStream{},
		Suggestions: components.NewSuggestions(&sty),
		ConfirmBar:  components.NewConfirmBar(&sty),
		Width:       80,
		Height:      24,
		state:       StateReady,
		confirmCh:   make(chan types.ConfirmRequest),
	}

	b.SetConfirmFn(func(req types.ConfirmRequest) bool {
		m.confirmCh <- req
		return <-req.Response
	})

	b.SetPhaseFn(func(phase string) {
		m.processingPhase = phase
	})

	used, budget := b.TokenUsage()
	m.Header.SetTokens(used, budget)

	return m
}

func (m *Model) Init() tea.Cmd {
	return m.Input.Init()
}

func (m *Model) resizeMessages() {
	extra := 0
	if m.state == StateConfirming {
		extra = m.ConfirmBar.Height()
	}
	m.Messages.SetSize(m.Width-1, m.Height-m.Header.Height()-m.Input.Height()-contentMarginV-extra)
}

func (m *Model) transitionTo(state ChatState) {
	m.state = state
	switch state {
	case StateReady:
		m.Messages.SetProcessing(false)
		m.Input.SetSending(false)
		m.ConfirmBar.Clear()
	case StateProcessing:
		m.processingStart = time.Now()
		m.processingPhase = "Thinking"
		m.Messages.SetProcessingStatus("Thinking")
		m.Stream.Reset()
		m.Messages.SetProcessing(true)
		m.Input.SetSending(true)
	case StateConfirming:
		// ConfirmBar request is set by the caller; spinner/view react to the state.
	}
	m.resizeMessages()
}

func listenConfirm(ch <-chan types.ConfirmRequest) tea.Cmd {
	return func() tea.Msg {
		req, ok := <-ch
		if !ok {
			return nil
		}
		return confirmMsg{req: req}
	}
}
