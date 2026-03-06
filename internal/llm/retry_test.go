package llm

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testNetError implements net.Error for testing transient network errors.
type testNetError struct {
	msg string
}

func (e *testNetError) Error() string   { return e.msg }
func (e *testNetError) Timeout() bool   { return true }
func (e *testNetError) Temporary() bool { return true }

// Verify it satisfies net.Error.
var _ net.Error = (*testNetError)(nil)

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "nil error",
			err:       nil,
			transient: false,
		},
		{
			name:      "net.Error is transient",
			err:       &testNetError{msg: "connection refused"},
			transient: true,
		},
		{
			name:      "rate limit 429",
			err:       fmt.Errorf("HTTP 429: rate limit exceeded"),
			transient: true,
		},
		{
			name:      "server error 500",
			err:       fmt.Errorf("HTTP 500: internal server error"),
			transient: true,
		},
		{
			name:      "bad gateway 502",
			err:       fmt.Errorf("HTTP 502: bad gateway"),
			transient: true,
		},
		{
			name:      "service unavailable 503",
			err:       fmt.Errorf("HTTP 503: service unavailable"),
			transient: true,
		},
		{
			name:      "anthropic overloaded 529",
			err:       fmt.Errorf("HTTP 529: overloaded"),
			transient: true,
		},
		{
			name:      "rate limit keyword",
			err:       fmt.Errorf("rate limit exceeded, please retry"),
			transient: true,
		},
		{
			name:      "overloaded keyword",
			err:       fmt.Errorf("API is overloaded"),
			transient: true,
		},
		{
			name:      "auth error 401 not transient",
			err:       fmt.Errorf("HTTP 401: unauthorized"),
			transient: false,
		},
		{
			name:      "invalid api key not transient",
			err:       fmt.Errorf("invalid api key"),
			transient: false,
		},
		{
			name:      "bad request 400 not transient",
			err:       fmt.Errorf("HTTP 400: bad request"),
			transient: false,
		},
		{
			name:      "generic error not transient",
			err:       fmt.Errorf("something went wrong"),
			transient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.transient, isTransient(tt.err))
		})
	}
}

func TestWithRetry(t *testing.T) {
	fastCfg := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		Multiplier:  2.0,
	}

	t.Run("immediate success", func(t *testing.T) {
		calls := 0
		result, err := withRetry(context.Background(), fastCfg, func() (string, error) {
			calls++
			return "ok", nil
		})

		assert.NoError(t, err)
		assert.Equal(t, "ok", result)
		assert.Equal(t, 1, calls)
	})

	t.Run("transient then success", func(t *testing.T) {
		calls := 0
		result, err := withRetry(context.Background(), fastCfg, func() (string, error) {
			calls++
			if calls == 1 {
				return "", &testNetError{msg: "connection reset"}
			}
			return "recovered", nil
		})

		assert.NoError(t, err)
		assert.Equal(t, "recovered", result)
		assert.Equal(t, 2, calls)
	})

	t.Run("permanent error no retry", func(t *testing.T) {
		calls := 0
		permErr := fmt.Errorf("invalid api key")
		_, err := withRetry(context.Background(), fastCfg, func() (string, error) {
			calls++
			return "", permErr
		})

		assert.ErrorIs(t, err, permErr)
		assert.Equal(t, 1, calls)
	})

	t.Run("all attempts exhausted", func(t *testing.T) {
		calls := 0
		transientErr := fmt.Errorf("HTTP 503: service unavailable")
		_, err := withRetry(context.Background(), fastCfg, func() (string, error) {
			calls++
			return "", transientErr
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all 3 attempts failed")
		assert.ErrorIs(t, err, transientErr)
		assert.Equal(t, 3, calls)
	})

	t.Run("context cancelled between retries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		_, err := withRetry(ctx, fastCfg, func() (string, error) {
			calls++
			if calls == 1 {
				cancel() // cancel context after first failure
				return "", fmt.Errorf("HTTP 503: service unavailable")
			}
			return "should not reach", nil
		})

		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, calls)
	})

	t.Run("rate limit error retried", func(t *testing.T) {
		calls := 0
		result, err := withRetry(context.Background(), fastCfg, func() (int, error) {
			calls++
			if calls < 3 {
				return 0, fmt.Errorf("HTTP 429: too many requests")
			}
			return 42, nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 42, result)
		assert.Equal(t, 3, calls)
	})

	t.Run("server errors retried", func(t *testing.T) {
		for _, code := range []string{"500", "502", "503"} {
			t.Run(code, func(t *testing.T) {
				calls := 0
				result, err := withRetry(context.Background(), fastCfg, func() (string, error) {
					calls++
					if calls == 1 {
						return "", fmt.Errorf("HTTP %s: server error", code)
					}
					return "ok", nil
				})

				assert.NoError(t, err)
				assert.Equal(t, "ok", result)
				assert.Equal(t, 2, calls)
			})
		}
	})

	t.Run("single attempt config", func(t *testing.T) {
		singleCfg := RetryConfig{
			MaxAttempts: 1,
			BaseDelay:   1 * time.Millisecond,
			Multiplier:  2.0,
		}
		calls := 0
		_, err := withRetry(context.Background(), singleCfg, func() (string, error) {
			calls++
			return "", fmt.Errorf("HTTP 503: service unavailable")
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all 1 attempts failed")
		assert.Equal(t, 1, calls)
	})
}
