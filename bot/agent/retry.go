package agent

import (
	"context"
	"fmt"
	"time"

	"primusbot/llm"
)

// withRetry executes fn with exponential backoff, logging retries to the debug log.
func withRetry(ctx context.Context, fn func() error) error {
	cfg := llm.DefaultRetryConfig
	var lastErr error
	for i := 0; i < cfg.MaxRetries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !llm.IsRetryable(err) {
			return err
		}
		if i == cfg.MaxRetries-1 {
			break
		}
		delay := cfg.BaseDelay * time.Duration(1<<i)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		writeAgentLog("retry %d/%d after %v: %v", i+1, cfg.MaxRetries, delay, err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", cfg.MaxRetries, lastErr)
}
