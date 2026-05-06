package components

import (
	"sync"

	"primusbot/tui/styles"

	tea "charm.land/bubbletea/v2"
)

type Messages struct {
	*List
	Processing     bool
	Follow         bool
	sty            *styles.Styles
	processingItem *ProcessingItem
	mu             sync.Mutex
}

func NewMessages(width, height int, sty *styles.Styles) *Messages {
	l := NewList()
	l.SetSize(width, height)
	l.SetGap(1)

	return &Messages{
		List:   l,
		Follow: true,
		sty:    sty,
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
		m.SetItems()
		for _, item := range items {
			if _, ok := item.(*ProcessingItem); !ok {
				m.AppendItems(item)
			}
		}
		m.processingItem = nil
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

func (m *Messages) SetProcessingStatus(text string) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetStatusText(text)
		m.Invalidate()
	}
	m.mu.Unlock()
}

func (m *Messages) SetProcessingTokens(prompt, completion int) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetTokens(prompt, completion)
	}
	m.mu.Unlock()
}

func (m *Messages) SetBlocks(blocks []ContentBlock) {
	m.mu.Lock()
	if m.processingItem != nil {
		m.processingItem.SetBlocks(blocks)
		m.Invalidate()
	}
	m.mu.Unlock()
}

func (m *Messages) AddMessage(msg ChatMessage) {
	item := msg.ToMessageItem(m.sty)
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

func (m *Messages) ToggleLastAssistant() {
	items := m.Items()
	for i := len(items) - 1; i >= 0; i-- {
		a, ok := items[i].(*AssistantMessageItem)
		if !ok || len(a.blocks) == 0 {
			continue
		}
		// If any tool block is collapsed, expand all; otherwise collapse all.
		expand := false
		for _, b := range a.blocks {
			if b.Type == BlockToolCall && b.Collapsed {
				expand = true
				break
			}
		}
		for j := range a.blocks {
			if a.blocks[j].Type == BlockToolCall {
				a.blocks[j].Collapsed = !expand
			}
		}
		m.Invalidate()
		return
	}
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
		switch mev := msg.Mouse(); mev.Button {
		case tea.MouseWheelUp:
			m.ScrollBy(-3)
		case tea.MouseWheelDown:
			m.ScrollBy(3)
		}
	}

	if m.AtBottom() {
		m.SetFollow(true)
	} else {
		m.SetFollow(false)
	}

	return m, nil
}

func (m *Messages) View() string {
	return m.Render()
}
