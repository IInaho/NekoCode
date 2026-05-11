// Input 消息输入框：封装 Bubble Tea textarea，含发送历史翻阅（historyActive 状态机）、
// 发送过渡态（sending prompt）、光标管理。
package components

import (
	"strings"
	"time"

	"nekocode/tui/styles"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)


const charLimit = 32768
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
	ta.CharLimit = charLimit
	ta.SetWidth(width)
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	// InsertNewline enabled so pasted multi-line text is preserved.
	// Enter key is intercepted in Update() to trigger submit instead of newline.
	ta.KeyMap.InsertNewline.SetEnabled(true)

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
	return strings.TrimRight(i.textarea.Value(), "\n\t\r ")
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

// Cursor returns the cursor position adjusted to be relative to the top of
// the Input's rendered view (not relative to the internal textarea).
// Callers only need to add the Input's absolute Y position in the full layout.
func (i *Input) Cursor() *tea.Cursor {
	c := i.textarea.Cursor()
	if c == nil {
		return nil
	}
	return tea.NewCursor(c.Position.X, 1)
}

func (i *Input) Update(msg tea.Msg) (*Input, tea.Cmd) {
	// Intercept Enter: don't insert newline (app handles submission).
	// Other keys (including paste events with multi-line content) pass through.
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "enter" {
		return i, nil
	}
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
		styles.TealStyle.Render(followText)
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
