package tui

import (
	"primusbot/bot"
	"primusbot/tui/components"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

type doneMsg struct {
	content          string
	reasoningContent string
	err              error
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
	completions   []string
	completionIdx int
}

const Version = "0.1.0"

func NewModel() *Model {
	b := bot.New()
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &Model{
		Bot:      b,
		Header:   components.NewHeader(80, b.Cfg.Provider, b.Cfg.Model, Version),
		Messages: components.NewMessages(80, 14),
		Input:    components.NewInput(80),
		Splash:   components.NewSplash(80, 24, Version),
		Spinner:  sp,
		Stream:   &StreamState{},
		Width:    80,
		Height:   24,
	}
}

func (m *Model) Init() tea.Cmd {
	return m.Input.Init()
}
