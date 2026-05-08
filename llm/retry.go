package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RetryConfig defines the exponential backoff parameters.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

var DefaultRetryConfig = RetryConfig{
	MaxRetries: 4,
	BaseDelay:  500 * time.Millisecond,
	MaxDelay:   8 * time.Second,
}

// IsRetryable classifies LLM errors into retryable vs terminal.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") || strings.Contains(msg, "504") {
		return true
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate") ||
		strings.Contains(msg, "too_many_requests") {
		return true
	}
	if strings.Contains(msg, "overloaded") || strings.Contains(msg, "exhausted") ||
		strings.Contains(msg, "unavailable") || strings.Contains(msg, "server error") ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "reset") ||
		strings.Contains(msg, "timeout") || strings.Contains(msg, "Timeout") {
		return true
	}
	if strings.Contains(msg, "HTTP 4") {
		return false
	}
	return true
}

// Retry executes fn with exponential backoff.
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error
	for i := 0; i < cfg.MaxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsRetryable(err) {
			return err
		}
		if i == cfg.MaxRetries-1 {
			break
		}
		delay := cfg.BaseDelay * time.Duration(1<<i)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", cfg.MaxRetries, lastErr)
}
