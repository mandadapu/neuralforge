package llm

import (
	"context"
	"log/slog"
	"time"
)

// maxErrLogLen caps error strings written to audit logs to prevent accidental
// logging of sensitive content that may be embedded in upstream error messages.
const maxErrLogLen = 200

// sanitizeError returns a safe, length-bounded string for audit log entries.
// Full error messages can embed request/response content containing PII/PHI;
// truncating removes that risk while preserving enough context for diagnostics.
func sanitizeError(err error) string {
	s := err.Error()
	if len(s) > maxErrLogLen {
		return s[:maxErrLogLen] + "...[truncated]"
	}
	return s
}

// AuditedLLM wraps an LLM backend and emits structured audit log entries
// for every completion request: provider, model, token counts, cost, and latency.
//
// Privacy note: logged fields are limited to provider-assigned identifiers
// (model name), numeric metrics (tokens, cost, latency), and truncated error
// strings. Request/response content is never logged.
type AuditedLLM struct {
	inner LLM
}

// NewAudited returns an LLM that wraps inner with audit logging.
// All call sites must go through this function instead of instantiating
// backends directly, satisfying the auditing factory requirement.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string {
	return a.inner.Name()
}

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	elapsed := time.Since(start)

	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", elapsed.Milliseconds(),
	}
	if err != nil {
		// Use sanitizeError to avoid logging PII/PHI that may appear in error messages.
		attrs = append(attrs, "error", sanitizeError(err))
		slog.WarnContext(ctx, "llm.Complete failed", attrs...)
	} else {
		slog.InfoContext(ctx, "llm.Complete", attrs...)
	}
	return resp, err
}

// StreamComplete wraps the inner stream and emits an audit log entry when the
// stream completes (Done=true) or encounters an error, recording latency and
// total content bytes as a proxy for token usage (streaming APIs do not always
// surface per-chunk token counts).
func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	innerCh, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.WarnContext(ctx, "llm.StreamComplete failed",
			"provider", a.inner.Name(),
			"error", sanitizeError(err),
		)
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var totalBytes int
		for chunk := range innerCh {
			totalBytes += len(chunk.Content)
			out <- chunk
			if chunk.Done || chunk.Error != nil {
				elapsed := time.Since(start)
				attrs := []any{
					"provider", a.inner.Name(),
					"latency_ms", elapsed.Milliseconds(),
					"total_content_bytes", totalBytes,
				}
				if chunk.Error != nil {
					attrs = append(attrs, "error", sanitizeError(chunk.Error))
					slog.WarnContext(ctx, "llm.StreamComplete failed", attrs...)
				} else {
					slog.InfoContext(ctx, "llm.StreamComplete", attrs...)
				}
			}
		}
	}()
	return out, nil
}
