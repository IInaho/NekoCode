package agent

import (
	"context"

	"nekocode/llm"
)

// withRetry executes fn with exponential backoff, logging retries to the debug log.
func withRetry(ctx context.Context, fn func() error) error {
	var attempt int
	return llm.Retry(ctx, llm.DefaultRetryConfig, func() error {
		err := fn()
		if err != nil && llm.IsRetryable(err) {
			attempt++
			writeAgentLog("retry %d: %v", attempt, err)
		}
		return err
	})
}
