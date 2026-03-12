package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM backend and emits structured audit log entries
// on every completion call: provider, model, token counts, cost, latency,
// and any error.
//
// Compliance notes:
//   - Legal basis for processing: legitimate interests (operational security and usage
//     monitoring). Prompt content is never logged; only metadata (token counts, cost,
//     latency) is recorded to satisfy data minimisation requirements.
//   - Error sanitisation: raw error messages are not logged to prevent inadvertent
//     capture of PII/PHI that may appear in provider error responses. Instead a safe
//     boolean indicator ("error_occurred") is emitted.
//   - Log retention and immutability: this wrapper emits to slog; write-once storage,
//     RBAC access controls, and retention periods must be enforced at the infrastructure
//     layer (e.g., append-only log sinks). SOX audit trail requirements cannot be met
//     solely in application code.
type AuditedLLM struct {
	inner LLM
}

// NewAuditedLLM returns an LLM that delegates to inner but logs each call.
// All LLM backends must be constructed through this factory so that audit
// controls cannot be bypassed.
func NewAuditedLLM(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

// sanitizeError returns a safe, non-PII representation of err for audit logs.
// The raw error message is not logged to prevent accidental PII/PHI exposure.
func sanitizeError(err error) any {
	if err == nil {
		return nil
	}
	return "error_occurred"
}

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	slog.Info("llm_call",
		"provider",      a.inner.Name(),
		"model",         resp.Model,
		"input_tokens",  resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd",      resp.Cost,
		"latency_ms",    time.Since(start).Milliseconds(),
		"error",         sanitizeError(err),
	)
	return resp, err
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	slog.Info("llm_stream_call_start", "provider", a.inner.Name())

	innerCh, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Info("llm_stream_call_end",
			"provider",   a.inner.Name(),
			"latency_ms", time.Since(start).Milliseconds(),
			"error",      sanitizeError(err),
		)
		return nil, err
	}

	auditCh := make(chan StreamChunk)
	go func() {
		defer close(auditCh)
		var chunkCount int
		var streamErr error
		for chunk := range innerCh {
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
			chunkCount++
			auditCh <- chunk
		}
		slog.Info("llm_stream_call_end",
			"provider",    a.inner.Name(),
			"chunk_count", chunkCount,
			"latency_ms",  time.Since(start).Milliseconds(),
			"error",       sanitizeError(streamErr),
		)
	}()
	return auditCh, nil
}
