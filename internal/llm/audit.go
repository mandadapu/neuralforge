package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps an LLM and emits structured audit log entries for every
// completion call, satisfying the security requirement that all LLM client
// usage passes through an auditing layer.
//
// Privacy / compliance notes:
//   - Message content (req.Messages[*].Content, req.System) is NEVER logged to
//     prevent exposure of PII, PHI, PAN, or other regulated data (GDPR Art. 5,
//     HIPAA Safe Harbor, PCI-DSS req. 3.3).
//   - Only structural metadata (message count, boolean flags, token counts,
//     costs, durations) is emitted; this is the minimum necessary for security
//     audit and SOX §404 operational-integrity evidence.
//   - Log consumers are responsible for ensuring log storage meets applicable
//     retention and tamper-protection requirements (SOX §404, HIPAA §164.312(b)).
type AuditedLLM struct {
	inner LLM
}

// NewAudited returns an LLM that wraps inner with audit logging.
// All callers must use this factory instead of instantiating backends directly.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	// NOTE: req.Messages content and req.System are intentionally omitted to
	// prevent PII/PHI/PAN from appearing in audit logs.
	slog.Info("llm.Complete started",
		"provider", a.inner.Name(),
		"model", req.Model,
		"message_count", len(req.Messages),
		"has_system", req.System != "",
	)

	resp, err := a.inner.Complete(ctx, req)

	// NOTE: resp.Content is intentionally omitted for the same PII reasons.
	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		slog.Error("llm.Complete failed", append(attrs, "error", err)...)
	} else {
		slog.Info("llm.Complete finished", attrs...)
	}
	return resp, err
}

// StreamComplete wraps the inner stream and emits a completion audit entry
// (with duration) when the stream finishes or errors, providing a full audit
// trail for SOX §404 purposes. Token counts and costs are not available from
// individual StreamChunks; use Complete for calls where those metrics are
// required.
func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	// NOTE: req.Model is the only request field logged; content is omitted.
	slog.Info("llm.StreamComplete started",
		"provider", a.inner.Name(),
		"model", req.Model,
	)
	innerCh, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm.StreamComplete failed",
			"provider", a.inner.Name(),
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for chunk := range innerCh {
			out <- chunk
			if chunk.Done || chunk.Error != nil {
				attrs := []any{
					"provider", a.inner.Name(),
					"duration_ms", time.Since(start).Milliseconds(),
				}
				if chunk.Error != nil {
					slog.Error("llm.StreamComplete finished with error", append(attrs, "error", chunk.Error)...)
				} else {
					slog.Info("llm.StreamComplete finished", attrs...)
				}
				return
			}
		}
	}()
	return out, nil
}
