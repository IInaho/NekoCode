package components

import (
	"strings"
	"time"

	"primusbot/ui/styles"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Input struct {
	textarea textarea.Model
	width    int
}

func NewInput(width int) *Input {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.SetVirtualCursor(false)
	ta.Focus()
	ta.Prompt = styles.GreenStyle.Render("┃ ")
	ta.CharLimit = 4096
	ta.SetWidth(width)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(s)

	return &Input{
		textarea: ta,
		width:    width,
	}
}

func (i *Input) SetWidth(width int) {
	i.width = width
	i.textarea.SetWidth(width)
}

func (i *Input) Width() int {
	return i.width
}

func (i *Input) Value() string {
	return strings.TrimSpace(i.textarea.Value())
}

func (i *Input) Reset() {
	i.textarea.Reset()
}

func (i *Input) Height() int {
	return 1
}

func (i *Input) Cursor() *tea.Cursor {
	return i.textarea.Cursor()
}

func (i *Input) Update(msg tea.Msg) (*Input, tea.Cmd) {
	var cmd tea.Cmd
	i.textarea, cmd = i.textarea.Update(msg)
	return i, cmd
}

func (i *Input) View() string {
	return i.textarea.View()
}

type TickMsg struct{}

func (i *Input) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tea.Every(time.Millisecond*500, func(t time.Time) tea.Msg {
			return TickMsg{}
		}),
	)
}
