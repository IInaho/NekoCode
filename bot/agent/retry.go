package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// retryConfig defines the exponential backoff parameters.
type retryConfig struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
}

var defaultRetry = retryConfig{
	maxRetries: 4,
	baseDelay:  500 * time.Millisecond,
	maxDelay:   8 * time.Second,
}

// isRetryable classifies LLM errors into retryable vs terminal.
// Mirrors opencode's retry policy: 5xx, network, rate-limit → retry; 4xx, context-cancel → don't.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := err.Error()
	// HTTP 5xx / server errors.
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") || strings.Contains(msg, "504") {
		return true
	}
	// Rate limit.
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate") ||
		strings.Contains(msg, "too_many_requests") {
		return true
	}
	// Transient server/network issues.
	if strings.Contains(msg, "overloaded") || strings.Contains(msg, "exhausted") ||
		strings.Contains(msg, "unavailable") || strings.Contains(msg, "server error") ||
		strings.Contains(msg, "connection refused") || strings.Contains(msg, "reset") ||
		strings.Contains(msg, "timeout") || strings.Contains(msg, "Timeout") {
		return true
	}
	// HTTP 4xx (non-rate-limit) → terminal.
	if strings.Contains(msg, "HTTP 4") {
		return false
	}
	// Unknown errors: retry once, but not indefinitely.
	return true
}

// withRetry executes fn with exponential backoff.
// Returns the last error if all retries are exhausted.
func withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for i := 0; i < defaultRetry.maxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			return err
		}
		if i == defaultRetry.maxRetries-1 {
			break
		}
		delay := defaultRetry.baseDelay * time.Duration(1<<i)
		if delay > defaultRetry.maxDelay {
			delay = defaultRetry.maxDelay
		}
		writeAgentLog("retry %d/%d after %v: %v", i+1, defaultRetry.maxRetries, delay, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", defaultRetry.maxRetries, lastErr)
}
