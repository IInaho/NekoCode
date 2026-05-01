package tui

import (
	"primusbot/tui/components"
	"sync"
)

// BlockStream is a thread-safe buffer of ContentBlocks.
// The agent goroutine appends blocks; the spinner tick reads snapshots.
type BlockStream struct {
	mu      sync.Mutex
	blocks  []components.ContentBlock
	seenIdx int
}

func (s *BlockStream) Reset() {
	s.mu.Lock()
	s.blocks = nil
	s.seenIdx = 0
	s.mu.Unlock()
}

func (s *BlockStream) Append(b components.ContentBlock) {
	s.mu.Lock()
	s.blocks = append(s.blocks, b)
	s.mu.Unlock()
}

func (s *BlockStream) Snapshot() []components.ContentBlock {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]components.ContentBlock, len(s.blocks))
	copy(out, s.blocks)
	return out
}

func (s *BlockStream) HasNew() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seenIdx < len(s.blocks)
}

func (s *BlockStream) MarkSeen() {
	s.mu.Lock()
	s.seenIdx = len(s.blocks)
	s.mu.Unlock()
}
