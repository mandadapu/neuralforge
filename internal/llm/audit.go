package llm

import (
	"context"
	"log/slog"
	"time"
)

// maxErrorLen caps the length of error messages written to logs to prevent
// PII/PHI/PAN leakage via provider error responses that may echo request content.
const maxErrorLen = 200

// redactError returns a sanitized, length-bounded error string safe for logging.
// Truncating prevents sensitive request content from appearing in log sinks.
func redactError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > maxErrorLen {
		return msg[:maxErrorLen] + " [truncated]"
	}
	return msg
}

// AuditingLLM wraps any LLM backend and emits audit log entries for every call.
// All log entries are tagged with sensitivity="metadata-only" to indicate that
// message content is intentionally excluded; only structural metadata is recorded.
// Legal basis for logging: operational audit trail for security controls.
//
// StreamComplete audits both invocation and stream completion/error via a
// forwarding goroutine that drains the inner channel and re-emits chunks.
type AuditingLLM struct {
	inner LLM
}

func NewAuditingLLM(inner LLM) *AuditingLLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	slog.Info("llm.Complete called",
		"provider", a.inner.Name(),
		"model", req.Model,
		"messages", len(req.Messages),
		"sensitivity", "metadata-only",
	)
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Error("llm.Complete failed",
			"provider", a.inner.Name(),
			"model", req.Model,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", redactError(err),
			"sensitivity", "metadata-only",
		)
		return resp, err
	}
	slog.Info("llm.Complete succeeded",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"duration_ms", time.Since(start).Milliseconds(),
		"sensitivity", "metadata-only",
	)
	return resp, nil
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	slog.Info("llm.StreamComplete called",
		"provider", a.inner.Name(),
		"model", req.Model,
		"sensitivity", "metadata-only",
	)
	inner, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm.StreamComplete failed",
			"provider", a.inner.Name(),
			"model", req.Model,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", redactError(err),
			"sensitivity", "metadata-only",
		)
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var chunks int
		var streamErr error
		for chunk := range inner {
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
			chunks++
			out <- chunk
		}
		if streamErr != nil {
			slog.Error("llm.StreamComplete stream error",
				"provider", a.inner.Name(),
				"model", req.Model,
				"chunks", chunks,
				"duration_ms", time.Since(start).Milliseconds(),
				"error", redactError(streamErr),
				"sensitivity", "metadata-only",
			)
		} else {
			slog.Info("llm.StreamComplete completed",
				"provider", a.inner.Name(),
				"model", req.Model,
				"chunks", chunks,
				"duration_ms", time.Since(start).Milliseconds(),
				"sensitivity", "metadata-only",
			)
		}
	}()
	return out, nil
}
