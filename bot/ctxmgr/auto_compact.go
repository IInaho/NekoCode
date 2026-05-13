// auto_compact.go — proactive context overflow detection and tiered compression.
// Called before every LLM call to prevent prompt_too_long errors.
// Uses 5 warning levels and a circuit breaker.

package ctxmgr

import (
	"fmt"
	"sync"
)

// CompactLevel defines the urgency of compression.
type CompactLevel int

const (
	LevelNormal    CompactLevel = iota // > 20K tokens buffer
	LevelWarning                       // ≤ 20K buffer — warn but don't act
	LevelMicroCompact                  // ≤ 13K buffer — trigger microCompact
	LevelCompact                       // ≤ 10K buffer — trigger full/session compact
	LevelBlocking                      // ≤ 3K buffer — refuse new input
)

func (l CompactLevel) String() string {
	switch l {
	case LevelNormal:
		return "normal"
	case LevelWarning:
		return "warning"
	case LevelMicroCompact:
		return "micro_compact"
	case LevelCompact:
		return "compact"
	case LevelBlocking:
		return "blocking"
	default:
		return "unknown"
	}
}

// AutoCompactConfig holds auto-compaction thresholds.
type AutoCompactConfig struct {
	WarningBuffer      int // tokens below which to warn (default: 20000)
	MicroCompactBuffer int // tokens below which to micro-compact (default: 13000)
	CompactBuffer      int // tokens below which to full compact (default: 10000)
	BlockingBuffer     int // tokens below which to block (default: 3000)
	MaxConsecutiveFail int // circuit breaker threshold (default: 3)
}

var DefaultAutoCompactConfig = AutoCompactConfig{
	WarningBuffer:      44800, // ~30% usage @64K
	MicroCompactBuffer: 35200, // ~45% usage @64K
	CompactBuffer:      25600, // ~60% usage @64K — LLM summary over blind clear
	BlockingBuffer:     6400,  // ~90% usage @64K
	MaxConsecutiveFail: 3,
}

// AutoCompactIfNeeded is the main entry point. It checks token usage and triggers
// the appropriate compression level. Called before each LLM call.
// Returns the action taken and any error.
func (m *Manager) AutoCompactIfNeeded(cfg AutoCompactConfig, sessionMemoryProvider func() string) (CompactLevel, error) {
	m.mu.RLock()
	estTokens := m.visibleEstimatedTokens()
	// Prefer API-calibrated token counts over pure heuristics
	// to avoid underestimating context pressure.
	if t := m.tokenTracker.Total(); t > estTokens {
		estTokens = t
	}
	effectiveBudget := m.tokenBudget
	m.mu.RUnlock()

	// Calculate remaining buffer.
	remaining := effectiveBudget - estTokens

	level := classifyLevel(remaining, cfg)
	if level == LevelNormal || level == LevelWarning {
		return level, nil
	}

	if level == LevelBlocking {
		return level, fmt.Errorf("context full: %d tokens used of %d budget (only %d remaining)",
			estTokens, effectiveBudget, remaining)
	}

	if level == LevelMicroCompact {
		cleared := m.MicroCompactIfNeeded()
		if cleared > 0 {
			return LevelMicroCompact, nil
		}
		// MicroCompact found nothing to clear — escalate to compact.
		level = LevelCompact
	}

	if level == LevelCompact {
		// Try session memory compression first.
		if sessionMemoryProvider != nil {
			if smContent := sessionMemoryProvider(); smContent != "" && len(smContent) > 100 {
				if err := m.SummarizeWithSessionMemory(smContent); err == nil {
					return LevelCompact, nil
				}
			}
		}

		if err := m.Summarize(); err != nil {
			return level, fmt.Errorf("auto compact failed: %w", err)
		}
		return LevelCompact, nil
	}

	return level, nil
}

func classifyLevel(remaining int, cfg AutoCompactConfig) CompactLevel {
	if remaining <= cfg.BlockingBuffer {
		return LevelBlocking
	}
	if remaining <= cfg.CompactBuffer {
		return LevelCompact
	}
	if remaining <= cfg.MicroCompactBuffer {
		return LevelMicroCompact
	}
	if remaining <= cfg.WarningBuffer {
		return LevelWarning
	}
	return LevelNormal
}

// TokenTracker provides accurate token counting by combining API usage data
// with heuristic estimation for new messages.
type TokenTracker struct {
	mu                sync.RWMutex
	lastPromptTokens  int // from last API response
	lastCompTokens    int
	newMessageTokens  int // estimated tokens in messages added since last API call
}

// RecordUsage records token usage from an API response.
// Only non-zero fields are updated — some providers (Anthropic) split
// prompt and completion usage across separate streaming events.
func (t *TokenTracker) RecordUsage(promptTokens, completionTokens int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if promptTokens > 0 {
		t.lastPromptTokens = promptTokens
	}
	if completionTokens > 0 {
		t.lastCompTokens = completionTokens
	}
	t.newMessageTokens = 0
}

// Total returns the current estimated token count.
func (t *TokenTracker) Total() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastPromptTokens + t.lastCompTokens + t.newMessageTokens
}

// AddNew estimates tokens for messages added since the last API call.
func (t *TokenTracker) AddNew(charCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.newMessageTokens += charCount / 4
}
