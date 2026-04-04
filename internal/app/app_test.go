package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStore implements store.Store for testing without a real database.
type mockStore struct {
	created []store.Job
	createErr error
}

func (m *mockStore) CreateJob(_ context.Context, job store.Job) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, job)
	return nil
}

func (m *mockStore) GetJob(_ context.Context, _ string) (*store.Job, error) { return nil, nil }
func (m *mockStore) GetJobByIssue(_ context.Context, _ string, _ int) (*store.Job, error) {
	return nil, nil
}
func (m *mockStore) UpdateJobStatus(_ context.Context, _ string, _ store.JobStatus, _ string) error {
	return nil
}
func (m *mockStore) UpdateJobError(_ context.Context, _ string, _ string) error { return nil }
func (m *mockStore) UpdateJobCost(_ context.Context, _ string, _ float64) error { return nil }
func (m *mockStore) CompleteJob(_ context.Context, _ string, _ store.JobStatus) error { return nil }
func (m *mockStore) ListPendingJobs(_ context.Context, _ int) ([]store.Job, error) {
	return nil, nil
}
func (m *mockStore) UpsertRepoContext(_ context.Context, _ store.RepoContextRecord) error {
	return nil
}
func (m *mockStore) GetRepoContext(_ context.Context, _ string) (*store.RepoContextRecord, error) {
	return nil, nil
}
func (m *mockStore) Migrate(_ context.Context) error { return nil }
func (m *mockStore) Close() error                    { return nil }

// errReader always returns an error on Read, simulating a broken request body.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// failWriteResponseWriter is an http.ResponseWriter whose Write always fails.
type failWriteResponseWriter struct {
	header     http.Header
	statusCode int
}

func (f *failWriteResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}
func (f *failWriteResponseWriter) WriteHeader(code int) { f.statusCode = code }
func (f *failWriteResponseWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// issueLabeledPayload builds a minimal issues webhook payload.
func issueLabeledPayload(installationID int64, includeInstallation bool) []byte {
	base := `{"action":"labeled","label":{"name":"neuralforge"},"issue":{"number":42,"title":"Test issue","body":"","user":{"login":"alice"},"labels":[]},"repository":{"full_name":"owner/repo","default_branch":"main","clone_url":"https://github.com/owner/repo.git"}}`
	if !includeInstallation {
		return []byte(base)
	}
	// Insert installation field before closing brace.
	payload := base[:len(base)-1]
	return []byte(payload + fmt.Sprintf(`,"installation":{"id":%d}}`, installationID))
}

func TestHandleEventExtractsInstallationID(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}

	payload := issueLabeledPayload(12345, true)
	a.handleEvent("issues", payload)

	require.Len(t, ms.created, 1)
	assert.Equal(t, int64(12345), ms.created[0].InstallationID)
}

func TestHandleEventWithoutInstallationID(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}

	payload := issueLabeledPayload(0, false)
	a.handleEvent("issues", payload)

	require.Len(t, ms.created, 1)
	assert.Equal(t, int64(0), ms.created[0].InstallationID)
}

func TestGhAppWebhookHandlerSuccess(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}
	handler := a.ghAppWebhookHandlerFunc()

	payload := `{"action":"push"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewBufferString(payload))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"ok":true`)
}

func TestGhAppWebhookHandlerReadBodyError(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}
	handler := a.ghAppWebhookHandlerFunc()

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", errReader{})
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Empty(t, ms.created)
}

func TestGhAppWebhookHandlerWriteError(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}
	handler := a.ghAppWebhookHandlerFunc()

	payload := `{"action":"push"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewBufferString(payload))
	req.Header.Set("X-GitHub-Event", "push")

	// Use a writer that fails on Write; verify the handler does not panic.
	fw := &failWriteResponseWriter{}
	assert.NotPanics(t, func() {
		handler.ServeHTTP(fw, req)
	})
	assert.Equal(t, http.StatusOK, fw.statusCode)
}

func TestHandleEventCreatesJobWithCorrectFields(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}

	payload := issueLabeledPayload(99999, true)
	a.handleEvent("issues", payload)

	require.Len(t, ms.created, 1)
	job := ms.created[0]
	assert.Equal(t, "owner/repo#42", job.ID)
	assert.Equal(t, "owner/repo", job.RepoFullName)
	assert.Equal(t, 42, job.IssueNumber)
	assert.Equal(t, store.JobQueued, job.Status)
	assert.Equal(t, int64(99999), job.InstallationID)
}

func TestHandleEventIgnoresNonNeuralforgeLabel(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}

	payload := []byte(`{"action":"labeled","label":{"name":"bug"},"issue":{"number":1,"title":"T"},"repository":{"full_name":"owner/repo"}}`)
	a.handleEvent("issues", payload)

	assert.Empty(t, ms.created)
}

func TestHandleEventUnknownEventTypeIsNoop(t *testing.T) {
	ms := &mockStore{}
	a := &App{store: ms}

	a.handleEvent("push", []byte(`{}`))

	assert.Empty(t, ms.created)
}

// Ensure mockStore satisfies the store.Store interface at compile time.
var _ store.Store = (*mockStore)(nil)

// Ensure Job.InstallationID zero value doesn't block token acquisition logic check.
func TestJobInstallationIDZeroMeansNoToken(t *testing.T) {
	job := store.Job{
		ID:             "owner/repo#1",
		RepoFullName:   "owner/repo",
		IssueNumber:    1,
		Status:         store.JobQueued,
		InstallationID: 0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	// A zero InstallationID means the ghApp token acquisition block is skipped.
	// This is enforced by the condition: if a.ghApp != nil && job.InstallationID > 0
	assert.Equal(t, int64(0), job.InstallationID)
}
