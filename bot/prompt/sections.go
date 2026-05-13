// sections.go — Section-level caching for dynamic system prompt parts.
//
// Pattern taken from Claude Code's systemPromptSection() /
// DANGEROUS_uncachedSystemPromptSection() design.
//
// Cached sections: computed once, only re-evaluated on /clear or /compact.
// These are stable across turns — putting them in the system prompt keeps
// the prompt cache warm.
//
// Uncached sections: recomputed every turn. These change frequently and
// would break the prompt cache if included as system prompt text. Use
// sparingly — each one costs a cache miss when its value changes.

package prompt

import "sync"

// SectionFunc computes the content of a system prompt section.
type SectionFunc func() string

type cachedSection struct {
	compute SectionFunc
	mu      sync.Mutex
	value   string
	valid   bool
}

type uncachedSection struct {
	compute SectionFunc
}

// CachedSection creates a section that is computed once and then cached.
// Cache is cleared by ClearAll() (called on /clear or /compact).
func CachedSection(compute SectionFunc) *cachedSection {
	return &cachedSection{compute: compute}
}

// Get returns the cached value, computing it on first call.
func (s *cachedSection) Get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.valid {
		s.value = s.compute()
		s.valid = true
	}
	return s.value
}

// Clear invalidates this section's cache so it will be recomputed.
func (s *cachedSection) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.valid = false
	s.value = ""
}

// UncachedSection creates a section that is recomputed on every call.
func UncachedSection(compute SectionFunc) *uncachedSection {
	return &uncachedSection{compute: compute}
}

// Get always recomputes the value.
func (s *uncachedSection) Get() string {
	return s.compute()
}

// SectionManager manages a collection of dynamic system prompt sections.
type SectionManager struct {
	cached   []*cachedSection
	uncached []*uncachedSection
}

// NewSectionManager creates a new SectionManager.
func NewSectionManager() *SectionManager {
	return &SectionManager{}
}

// AddCached registers a cached section.
func (sm *SectionManager) AddCached(s *cachedSection) {
	sm.cached = append(sm.cached, s)
}

// AddUncached registers an uncached section.
func (sm *SectionManager) AddUncached(s *uncachedSection) {
	sm.uncached = append(sm.uncached, s)
}

// BuildDynamicSuffix assembles all dynamic sections in order.
// Returns the assembled text for appending after the static prefix.
func (sm *SectionManager) BuildDynamicSuffix() string {
	if len(sm.cached) == 0 && len(sm.uncached) == 0 {
		return ""
	}
	var result string
	for _, s := range sm.cached {
		if v := s.Get(); v != "" {
			if result != "" {
				result += "\n\n"
			}
			result += v
		}
	}
	for _, s := range sm.uncached {
		if v := s.Get(); v != "" {
			if result != "" {
				result += "\n\n"
			}
			result += v
		}
	}
	return result
}

// ClearAllCached invalidates all cached sections.
func (sm *SectionManager) ClearAllCached() {
	for _, s := range sm.cached {
		s.Clear()
	}
}
