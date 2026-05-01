package components

import (
	"strings"

	"primusbot/tui/styles"
)

type Suggestions struct {
	items       []string
	selectedIdx int
	visible     bool
	sty         *styles.Styles
}

func NewSuggestions(sty *styles.Styles) *Suggestions {
	return &Suggestions{sty: sty}
}

func (s *Suggestions) Refresh(prefix string, commands []string) {
	s.items = nil
	s.selectedIdx = 0
	s.visible = false

	if !strings.HasPrefix(prefix, "/") {
		return
	}

	p := strings.TrimPrefix(prefix, "/")
	for _, name := range commands {
		if strings.HasPrefix(name, p) {
			s.items = append(s.items, "/"+name)
		}
	}
	if len(s.items) == 1 && s.items[0] == prefix {
		return
	}
	if len(s.items) > 0 {
		s.visible = true
	}
}

func (s *Suggestions) Accept() string {
	if !s.visible || len(s.items) == 0 {
		return ""
	}
	val := s.items[s.selectedIdx]
	s.visible = false
	return val
}

func (s *Suggestions) Cycle(delta int) {
	if !s.visible || len(s.items) == 0 {
		return
	}
	s.selectedIdx = (s.selectedIdx + delta + len(s.items)) % len(s.items)
}

func (s *Suggestions) Visible() bool { return s.visible }
func (s *Suggestions) Hide()         { s.visible = false }

func (s *Suggestions) View(width int) string {
	if !s.visible || len(s.items) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(s.sty.Subtle.Render("── suggestions ──") + "\n")
	for i, item := range s.items {
		if i == s.selectedIdx {
			b.WriteString(s.sty.Primary.Bold(true).Render("> " + item) + "\n")
		} else {
			b.WriteString(s.sty.Muted.Render("  " + item) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

