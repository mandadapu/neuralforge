package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"strings"
	"time"
)

// RetryConfig controls retry behavior for LLM API calls.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	Multiplier  float64
}

// DefaultRetryConfig provides sensible defaults: 3 attempts with exponential backoff
// starting at 500ms (delays: 500ms, 1s, then fail).
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   500 * time.Millisecond,
	Multiplier:  2.0,
}

// isTransient returns true if the error is likely transient and worth retrying.
func isTransient(err error) bool {
	if err == nil {
		return false
	}

	// Network errors (timeout, connection refused, DNS)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Rate limit and server errors detected by message content.
	// Both anthropic-sdk-go and openai-go wrap HTTP errors with status codes.
	msg := strings.ToLower(err.Error())
	for _, substr := range []string{"429", "500", "502", "503", "529", "rate", "overloaded"} {
		if strings.Contains(msg, substr) {
			return true
		}
	}

	return false
}

// withRetry executes fn up to cfg.MaxAttempts times, retrying on transient errors
// with exponential backoff. Permanent errors are returned immediately.
func withRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !isTransient(err) {
			return zero, err
		}

		if attempt < cfg.MaxAttempts-1 {
			delay := time.Duration(float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
			log.Printf("llm: transient error (attempt %d/%d), retrying in %v: %v",
				attempt+1, cfg.MaxAttempts, delay, err)

			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return zero, fmt.Errorf("all %d attempts failed: %w", cfg.MaxAttempts, lastErr)
}
