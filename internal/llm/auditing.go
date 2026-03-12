package llm

import (
	"context"
	"log/slog"
	"time"
)

// correlationIDKey is the unexported context key for correlation ID propagation.
type correlationIDKey struct{}

// WithCorrelationID returns a new context carrying the given correlation ID.
// Callers (e.g. job handlers) should attach an ID before calling any LLM method
// so audit logs can be tied back to the originating business transaction.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// correlationIDFromContext extracts the correlation ID from ctx; returns "" if absent.
func correlationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey{}).(string); ok {
		return id
	}
	return ""
}

// AuditingLLM wraps any LLM backend and emits structured audit log entries
// for every completion call, capturing provider, model, token usage, latency,
// correlation ID, and any error.
//
// Prompt text and response content are intentionally excluded from all log
// fields to prevent inadvertent capture of PII/PHI in log sinks.
type AuditingLLM struct {
	inner LLM
}

// NewAuditingLLM wraps inner with audit logging. It is the sole approved way
// to obtain an LLM instance for production use.
func NewAuditingLLM(inner LLM) *AuditingLLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	latency := time.Since(start)
	corrID := correlationIDFromContext(ctx)
	if err != nil {
		slog.Error("llm complete",
			"provider", a.inner.Name(),
			"model", req.Model,
			"latency_ms", latency.Milliseconds(),
			"correlation_id", corrID,
			"error", err,
		)
	} else {
		slog.Info("llm complete",
			"provider", a.inner.Name(),
			"model", resp.Model,
			"input_tokens", resp.InputTokens,
			"output_tokens", resp.OutputTokens,
			"cost_usd", resp.Cost,
			"latency_ms", latency.Milliseconds(),
			"correlation_id", corrID,
		)
	}
	return resp, err
}

// StreamComplete starts a streaming completion and returns a wrapped channel.
// A goroutine monitors the stream and emits a deferred audit event when the
// stream closes, capturing chunk count, latency, and any stream-level error.
// Content is not logged to avoid PII capture.
func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	corrID := correlationIDFromContext(ctx)

	slog.Info("llm stream_complete start",
		"provider", a.inner.Name(),
		"model", req.Model,
		"correlation_id", corrID,
	)

	innerCh, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm stream_complete",
			"provider", a.inner.Name(),
			"model", req.Model,
			"latency_ms", time.Since(start).Milliseconds(),
			"correlation_id", corrID,
			"error", err,
		)
		return nil, err
	}

	outCh := make(chan StreamChunk)
	go func() {
		defer close(outCh)
		var chunks int
		var finalErr error
		for chunk := range innerCh {
			if chunk.Error != nil {
				finalErr = chunk.Error
			}
			outCh <- chunk
			chunks++
		}
		latency := time.Since(start)
		if finalErr != nil {
			slog.Error("llm stream_complete",
				"provider", a.inner.Name(),
				"model", req.Model,
				"chunks", chunks,
				"latency_ms", latency.Milliseconds(),
				"correlation_id", corrID,
				"error", finalErr,
			)
		} else {
			slog.Info("llm stream_complete",
				"provider", a.inner.Name(),
				"model", req.Model,
				"chunks", chunks,
				"latency_ms", latency.Milliseconds(),
				"correlation_id", corrID,
			)
		}
	}()

	return outCh, nil
}
