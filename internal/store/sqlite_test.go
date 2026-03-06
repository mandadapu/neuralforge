package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetJob(t *testing.T) {
	s := newTestStore(t)
	job := Job{ID: "job-1", RepoFullName: "owner/repo", IssueNumber: 42, IssueTitle: "Fix bug", Status: JobQueued}
	require.NoError(t, s.CreateJob(job))

	got, err := s.GetJob("job-1")
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", got.RepoFullName)
	assert.Equal(t, 42, got.IssueNumber)
	assert.Equal(t, JobQueued, got.Status)
}

func TestGetJobByIssue(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(Job{ID: "job-2", RepoFullName: "owner/repo", IssueNumber: 7, Status: JobQueued}))

	got, err := s.GetJobByIssue("owner/repo", 7)
	require.NoError(t, err)
	assert.Equal(t, "job-2", got.ID)
}

func TestUpdateJobStatus(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(Job{ID: "job-3", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued}))
	require.NoError(t, s.UpdateJobStatus("job-3", JobRunning, "architect"))

	got, err := s.GetJob("job-3")
	require.NoError(t, err)
	assert.Equal(t, JobRunning, got.Status)
	assert.Equal(t, "architect", got.CurrentStage)
}

func TestListPendingJobs(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(Job{ID: "j1", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued}))
	require.NoError(t, s.CreateJob(Job{ID: "j2", RepoFullName: "o/r", IssueNumber: 2, Status: JobRunning}))
	require.NoError(t, s.CreateJob(Job{ID: "j3", RepoFullName: "o/r", IssueNumber: 3, Status: JobCompleted}))

	jobs, err := s.ListPendingJobs(10)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "j1", jobs[0].ID)
}

func TestCompleteJob(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.CreateJob(Job{ID: "j4", RepoFullName: "o/r", IssueNumber: 4, Status: JobRunning}))
	require.NoError(t, s.CompleteJob("j4", JobCompleted))

	got, err := s.GetJob("j4")
	require.NoError(t, err)
	assert.Equal(t, JobCompleted, got.Status)
	assert.NotNil(t, got.CompletedAt)
}
