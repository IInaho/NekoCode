// Input 消息输入框：封装 Bubble Tea textarea，含发送历史翻阅（historyActive 状态机）、
// 发送过渡态（sending prompt）、光标管理。
package components

import (
	"strings"
	"time"

	"primusbot/tui/styles"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Input struct {
	textarea      textarea.Model
	width         int
	follow        bool
	sending       bool
	history       []string
	historyIdx    int
	savedInput    string
	historyActive bool
}

func NewInput(width int) *Input {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.SetVirtualCursor(false)
	ta.Focus()
	ta.Prompt = styles.CatEyeStyle.Bold(true).Render(styles.HeavyVert + " ")
	ta.CharLimit = 4096
	ta.SetWidth(width)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	s := ta.Styles()
	s.Focused.CursorLine = lipgloss.NewStyle()
	s.Focused.Placeholder = styles.MutedStyle
	s.Blurred.Placeholder = styles.MutedStyle
	ta.SetStyles(s)

	return &Input{
		textarea: ta,
		width:    width,
		follow:   true,
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

func (i *Input) SetValue(value string) {
	i.textarea.SetValue(value)
}

func (i *Input) SetCursorEnd() {
	i.textarea.SetCursorColumn(len(i.textarea.Value()))
}

func (i *Input) Reset() {
	i.textarea.Reset()
	i.sending = false
	i.historyActive = false
	i.textarea.Prompt = styles.CatEyeStyle.Bold(true).Render(styles.HeavyVert + " ")
}

func (i *Input) AddHistory(entry string) {
	if entry == "" {
		return
	}
	if len(i.history) > 0 && i.history[len(i.history)-1] == entry {
		return
	}
	i.history = append(i.history, entry)
	i.historyIdx = len(i.history)
}

func (i *Input) HistoryUp() {
	if len(i.history) == 0 {
		return
	}
	if i.historyIdx == len(i.history) {
		i.savedInput = i.textarea.Value()
	}
	if i.historyIdx > 0 {
		i.historyIdx--
		i.textarea.SetValue(i.history[i.historyIdx])
	}
	i.historyActive = true
}

func (i *Input) HistoryDown() {
	if i.historyIdx >= len(i.history) {
		return
	}
	i.historyIdx++
	if i.historyIdx == len(i.history) {
		i.textarea.SetValue(i.savedInput)
		i.historyActive = false
	} else {
		i.textarea.SetValue(i.history[i.historyIdx])
	}
}


func (i *Input) SetSending(sending bool) {
	i.sending = sending
	if sending {
		i.textarea.Prompt = styles.MutedStyle.Render("⋯ ")
	} else {
		i.textarea.Prompt = styles.CatEyeStyle.Bold(true).Render(styles.HeavyVert + " ")
	}
}

func (i *Input) SetFollow(follow bool) {
	i.follow = follow
}

func (i *Input) Height() int {
	return 5 // separator + input + spacer + footer + separator
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
	w := max(20, i.width)
	line := strings.Repeat(styles.Horizontal, w)

	followText := "Auto"
	if !i.follow {
		followText = "Manual"
	}

	footer := styles.BorderStyle.Render(styles.Vertical+" ") +
		styles.SubtleStyle.Render("Follow:") + " " +
		styles.GreenStyle.Render(followText)
	footerW := lipgloss.Width(footer)
	end := styles.BorderStyle.Render(styles.Vertical)
	pad := max(0, w-footerW-lipgloss.Width(end))

	var b strings.Builder
	b.WriteString(styles.BorderStyle.Render(line) + "\n")
	b.WriteString(i.textarea.View() + "\n")
	b.WriteString("\n")
	b.WriteString(footer + strings.Repeat(" ", pad) + end + "\n")
	b.WriteString(styles.BorderStyle.Render(line))

	return b.String()
}

type TickMsg struct{}

func (i *Input) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, BlinkTick())
}

func BlinkTick() tea.Cmd {
	return tea.Every(time.Millisecond*500, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}
