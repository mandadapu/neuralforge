package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM backend and emits a structured audit log entry
// for every completion call, recording provider, model, token usage, cost,
// and latency. All LLM usage must flow through this wrapper so that the audit
// trail is never bypassed.
type AuditedLLM struct {
	inner LLM
}

// NewAudited wraps inner in an AuditedLLM. All callers that previously
// instantiated a backend directly (NewOpenAI, NewClaude) must instead call
// NewAudited(NewOpenAI(...)) or NewAudited(NewClaude(...)).
func NewAudited(inner LLM) *AuditedLLM {
	if inner == nil {
		panic("llm.NewAudited: inner must not be nil")
	}
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	latency := time.Since(start)

	if err != nil {
		slog.Info("llm.complete",
			"provider", a.inner.Name(),
			"model", req.Model,
			"latency_ms", latency.Milliseconds(),
			"error", err,
		)
		return resp, err
	}

	slog.Info("llm.complete",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", latency.Milliseconds(),
	)
	return resp, nil
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm.stream_complete",
		"provider", a.inner.Name(),
		"model", req.Model,
	)
	return a.inner.StreamComplete(ctx, req)
}
