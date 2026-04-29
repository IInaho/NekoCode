package components

import (
	"strings"
	"sync"

	"primusbot/tui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	scrollbarThumb = styles.HeavyVert
	scrollbarTrack = styles.Vertical
)

type Messages struct {
	*List
	Processing     bool
	Follow         bool
	sty            *styles.Styles
	processingItem *ProcessingItem
	mu             sync.Mutex
}

func NewMessages(width, height int) *Messages {
	sty := styles.DefaultStyles()
	l := NewList()
	l.SetSize(width, height)
	l.SetGap(1)

	return &Messages{
		List:   l,
		Follow: true,
		sty:    &sty,
	}
}

func (m *Messages) SetSize(width, height int) {
	m.List.SetSize(width, height)
}

func (m *Messages) SetProcessing(processing bool) {
	m.mu.Lock()
	m.Processing = processing
	if processing && m.processingItem == nil {
		m.processingItem = NewProcessingItem(m.sty)
		m.AppendItems(m.processingItem)
	} else if !processing && m.processingItem != nil {
		items := m.Items()
		m.List.SetItems()
		for _, item := range items {
			if _, ok := item.(*ProcessingItem); !ok {
				m.AppendItems(item)
			}
		}
		m.processingItem = nil
	}
	m.mu.Unlock()
}

func (m *Messages) SetStreamText(text string) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetStreamText(text)
		m.List.Invalidate()
	}
	m.mu.Unlock()
}

func (m *Messages) SetReasoningText(text string) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetThinkingText(text)
		m.List.Invalidate()
	}
	m.mu.Unlock()
}

func (m *Messages) SetStreamContentWidth(width int) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetContentWidth(width)
	}
	m.mu.Unlock()
}

func (m *Messages) SetSpinnerView(view string) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetSpinnerView(view)
	}
	m.mu.Unlock()
}

func (m *Messages) AddMessage(msg ChatMessage) {
	id := generateID(m.Len())
	item := msg.ToMessageItem(m.sty, id)
	m.AppendItems(item)
	if m.Follow {
		m.ScrollToBottom()
	}
}

func (m *Messages) SetFollow(follow bool) {
	m.mu.Lock()
	m.Follow = follow
	m.mu.Unlock()
}

func (m *Messages) GotoBottom() {
	m.ScrollToBottom()
	m.SetFollow(true)
}

func (m *Messages) Update(msg tea.Msg) (*Messages, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up":
			m.ScrollBy(-1)
		case "down":
			m.ScrollBy(1)
		case "pgup":
			m.ScrollBy(-m.Height())
		case "pgdown":
			m.ScrollBy(m.Height())
		}
	case tea.MouseMsg:
	}

	if m.AtBottom() {
		m.SetFollow(true)
	} else {
		m.SetFollow(false)
	}

	return m, nil
}

func (m *Messages) View() string {
	content := m.List.Render()
	scrollbar := m.renderScrollbar()

	if scrollbar == "" {
		return content
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, content, scrollbar)
}

func (m *Messages) renderScrollbar() string {
	totalHeight := m.TotalContentHeight()
	viewportHeight := m.Height()

	if totalHeight <= viewportHeight {
		return ""
	}

	scrollPercent := m.ScrollPercent()
	thumbSize := max(1, viewportHeight*viewportHeight/totalHeight)

	thumbPos := 0
	trackSpace := viewportHeight - thumbSize
	if trackSpace > 0 {
		thumbPos = min(trackSpace, int(float64(trackSpace)*scrollPercent))
	}

	var sb strings.Builder
	for i := 0; i < viewportHeight; i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(m.sty.Scrollbar.Thumb.Render(scrollbarThumb))
		} else {
			sb.WriteString(m.sty.Scrollbar.Track.Render(scrollbarTrack))
		}
	}

	return sb.String()
}

func generateID(index int) string {
	const digits = "abcdefghijklmnopqrstuvwxyz0123456789"
	if index < len(digits) {
		return string(digits[index])
	}
	return generateID(index/len(digits)) + string(digits[index%len(digits)])
}
