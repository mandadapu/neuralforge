package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp creates an App with an in-memory SQLite store for testing handleEvent.
func newTestApp(t *testing.T) *App {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { s.Close() })
	return &App{store: s}
}

func issueLabeledPayload(t *testing.T, label string, issueNum int, issueTitle, repoFullName string) []byte {
	t.Helper()
	payload := map[string]any{
		"action": "labeled",
		"label":  map[string]any{"name": label},
		"issue": map[string]any{
			"number": issueNum,
			"title":  issueTitle,
			"body":   "",
			"user":   map[string]any{"login": "author"},
			"labels": []any{},
		},
		"repository": map[string]any{
			"full_name":      repoFullName,
			"default_branch": "main",
			"clone_url":      "https://github.com/" + repoFullName + ".git",
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func TestHandleEvent_IssueLabeledNeuralforge(t *testing.T) {
	a := newTestApp(t)

	payload := issueLabeledPayload(t, "neuralforge", 42, "Fix the bug", "owner/repo")
	a.handleEvent("issues", payload)

	ctx := context.Background()
	job, err := a.store.GetJobByIssue(ctx, "owner/repo", 42)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "owner/repo#42", job.ID)
	assert.Equal(t, store.JobQueued, job.Status)
	assert.Equal(t, "Fix the bug", job.IssueTitle)
}

func TestHandleEvent_IssueLabeledOtherLabel(t *testing.T) {
	a := newTestApp(t)

	payload := issueLabeledPayload(t, "bug", 7, "Some issue", "owner/repo")
	a.handleEvent("issues", payload)

	ctx := context.Background()
	job, err := a.store.GetJobByIssue(ctx, "owner/repo", 7)
	require.NoError(t, err)
	assert.Nil(t, job, "no job should be created for non-neuralforge labels")
}

func TestHandleEvent_UnsupportedEventType(t *testing.T) {
	a := newTestApp(t)

	// "push" events are not handled — should return without panicking
	a.handleEvent("push", []byte(`{"ref":"refs/heads/main"}`))

	ctx := context.Background()
	jobs, err := a.store.ListPendingJobs(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestHandleEvent_MalformedPayload(t *testing.T) {
	a := newTestApp(t)

	// Malformed JSON should not panic
	assert.NotPanics(t, func() {
		a.handleEvent("issues", []byte(`{invalid json`))
	})
}

func TestHandleEvent_IssueNotLabeled(t *testing.T) {
	a := newTestApp(t)

	// action != "labeled" — no job should be created
	payload := map[string]any{
		"action": "opened",
		"issue": map[string]any{
			"number": 5,
			"title":  "New issue",
			"body":   "",
			"user":   map[string]any{"login": "someone"},
			"labels": []any{},
		},
		"repository": map[string]any{
			"full_name":      "owner/repo",
			"default_branch": "main",
			"clone_url":      "https://github.com/owner/repo.git",
		},
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)

	a.handleEvent("issues", b)

	ctx := context.Background()
	job, err := a.store.GetJobByIssue(ctx, "owner/repo", 5)
	require.NoError(t, err)
	assert.Nil(t, job)
}
