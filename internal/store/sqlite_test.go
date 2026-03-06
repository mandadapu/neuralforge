package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetJob(t *testing.T) {
	s := newTestStore(t)
	job := Job{ID: "job-1", RepoFullName: "owner/repo", IssueNumber: 42, IssueTitle: "Fix bug", Status: JobQueued}
	require.NoError(t, s.CreateJob(context.Background(), job))

	got, err := s.GetJob(context.Background(), "job-1")
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", got.RepoFullName)
	assert.Equal(t, 42, got.IssueNumber)
	assert.Equal(t, JobQueued, got.Status)
}

func TestGetJobByIssue(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(context.Background(), Job{ID: "job-2", RepoFullName: "owner/repo", IssueNumber: 7, Status: JobQueued}))

	got, err := s.GetJobByIssue(context.Background(), "owner/repo", 7)
	require.NoError(t, err)
	assert.Equal(t, "job-2", got.ID)
}

func TestUpdateJobStatus(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(context.Background(), Job{ID: "job-3", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued}))
	require.NoError(t, s.UpdateJobStatus(context.Background(), "job-3", JobRunning, "architect"))

	got, err := s.GetJob(context.Background(), "job-3")
	require.NoError(t, err)
	assert.Equal(t, JobRunning, got.Status)
	assert.Equal(t, "architect", got.CurrentStage)
}

func TestListPendingJobs(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(context.Background(), Job{ID: "j1", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued}))
	require.NoError(t, s.CreateJob(context.Background(), Job{ID: "j2", RepoFullName: "o/r", IssueNumber: 2, Status: JobRunning}))
	require.NoError(t, s.CreateJob(context.Background(), Job{ID: "j3", RepoFullName: "o/r", IssueNumber: 3, Status: JobCompleted}))

	jobs, err := s.ListPendingJobs(context.Background(), 10)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "j1", jobs[0].ID)
}

func TestCompleteJob(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(context.Background(), Job{ID: "j4", RepoFullName: "o/r", IssueNumber: 4, Status: JobRunning}))
	require.NoError(t, s.CompleteJob(context.Background(), "j4", JobCompleted))

	got, err := s.GetJob(context.Background(), "j4")
	require.NoError(t, err)
	assert.Equal(t, JobCompleted, got.Status)
	assert.NotNil(t, got.CompletedAt)
}

func TestContextCancellation(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err := s.CreateJob(ctx, Job{ID: "x", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued})
	assert.Error(t, err)
}
