package tui

import (
	"primusbot/tui/components"
	"sync"
	"unicode/utf8"
)

type BlockStream struct {
	mu        sync.Mutex
	blocks    []components.ContentBlock
	seenIdx   int
	backlog   string
	displayed string
}

func (s *BlockStream) Reset() {
	s.mu.Lock()
	s.blocks = nil
	s.seenIdx = 0
	s.backlog = ""
	s.displayed = ""
	s.mu.Unlock()
}

func (s *BlockStream) Append(b components.ContentBlock) {
	s.mu.Lock()
	s.blocks = append(s.blocks, b)
	s.backlog = ""
	s.displayed = ""
	s.mu.Unlock()
}

func (s *BlockStream) AppendText(delta string) {
	s.mu.Lock()
	s.backlog += delta
	s.mu.Unlock()
}

func (s *BlockStream) Snapshot() []components.ContentBlock {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.backlog) > 0 {
		n := utf8.RuneCountInString(s.backlog)
		release := max(1, min(n/3, 15))
		for i := 0; i < release && len(s.backlog) > 0; i++ {
			r, size := utf8.DecodeRuneInString(s.backlog)
			if r == utf8.RuneError {
				break
			}
			s.displayed += s.backlog[:size]
			s.backlog = s.backlog[size:]
		}
	}

	out := make([]components.ContentBlock, len(s.blocks))
	copy(out, s.blocks)
	if s.displayed != "" || s.backlog != "" {
		out = append(out, components.ContentBlock{
			Type:    components.BlockText,
			Content: s.displayed,
		})
	}
	return out
}

func (s *BlockStream) Dirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.backlog) > 0 || s.seenIdx < len(s.blocks)
}

func (s *BlockStream) MarkSeen() {
	s.mu.Lock()
	s.seenIdx = len(s.blocks)
	s.mu.Unlock()
}

func (s *BlockStream) Finalize() []components.ContentBlock {
	s.mu.Lock()
	s.displayed += s.backlog
	s.backlog = ""
	out := make([]components.ContentBlock, len(s.blocks))
	copy(out, s.blocks)
	s.displayed = ""
	s.mu.Unlock()
	return out
}
