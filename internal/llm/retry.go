package llm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// cbThreshold is the number of consecutive failures before the circuit opens.
	cbThreshold = 5
	// cbCooldown is how long the circuit stays open before allowing retries.
	cbCooldown = 2 * time.Minute
)

// circuitBreakerLLM wraps an LLM backend with a simple circuit breaker that
// stops forwarding requests after cbThreshold consecutive failures, preventing
// cascading failures in downstream systems.
type circuitBreakerLLM struct {
	backend   LLM
	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

// WrapCircuitBreaker wraps an LLM with a circuit breaker. After cbThreshold
// consecutive errors the circuit opens for cbCooldown; it resets on success.
func WrapCircuitBreaker(backend LLM) LLM {
	return &circuitBreakerLLM{backend: backend}
}

func (cb *circuitBreakerLLM) Name() string { return cb.backend.Name() }

func (cb *circuitBreakerLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	cb.mu.Lock()
	if time.Now().Before(cb.openUntil) {
		cb.mu.Unlock()
		return CompletionResponse{}, fmt.Errorf("llm circuit breaker open: provider=%s", cb.backend.Name())
	}
	cb.mu.Unlock()

	resp, err := cb.backend.Complete(ctx, req)

	cb.mu.Lock()
	defer cb.mu.Unlock()
	if err != nil {
		cb.failures++
		if cb.failures >= cbThreshold {
			cb.openUntil = time.Now().Add(cbCooldown)
			log.Printf("llm: circuit breaker opened for %s after %d consecutive failures", cb.backend.Name(), cb.failures)
		}
	} else {
		cb.failures = 0
	}
	return resp, err
}

func (cb *circuitBreakerLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	cb.mu.Lock()
	if time.Now().Before(cb.openUntil) {
		cb.mu.Unlock()
		return nil, fmt.Errorf("llm circuit breaker open: provider=%s", cb.backend.Name())
	}
	cb.mu.Unlock()
	return cb.backend.StreamComplete(ctx, req)
}

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
