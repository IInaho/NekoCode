package tui

import (
	"strings"
	"sync"
)

// StreamState encapsulates the mutable streaming state with safe concurrent access.
type StreamState struct {
	mu        sync.Mutex
	text      strings.Builder
	reasoning strings.Builder
	active    bool
	lastLen   int
}

func (s *StreamState) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *StreamState) Start() {
	s.mu.Lock()
	s.text.Reset()
	s.reasoning.Reset()
	s.active = true
	s.lastLen = 0
	s.mu.Unlock()
}

func (s *StreamState) Stop() {
	s.mu.Lock()
	s.active = false
	s.mu.Unlock()
}

func (s *StreamState) Append(content, reasoning string) {
	s.mu.Lock()
	if content != "" {
		s.text.WriteString(content)
	}
	if reasoning != "" {
		s.reasoning.WriteString(reasoning)
	}
	s.mu.Unlock()
}

// Snapshot returns the current text and reasoning content.
func (s *StreamState) Snapshot() (text, reasoning string) {
	s.mu.Lock()
	text = s.text.String()
	reasoning = s.reasoning.String()
	s.mu.Unlock()
	return
}

// HasNew returns true if new content arrived since last MarkSeen.
func (s *StreamState) HasNew() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.text.String()) > s.lastLen || s.reasoning.Len() > 0
}

// MarkSeen records the current length so HasNew returns false until more data arrives.
func (s *StreamState) MarkSeen() {
	s.mu.Lock()
	s.lastLen = len(s.text.String())
	s.mu.Unlock()
}
