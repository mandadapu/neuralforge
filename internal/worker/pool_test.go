package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	store.Store
	jobs    []store.Job
	updated int32
}

func (m *mockStore) ListPendingJobs(ctx context.Context, limit int) ([]store.Job, error) {
	if len(m.jobs) == 0 {
		return nil, nil
	}
	j := m.jobs[0]
	m.jobs = m.jobs[1:]
	return []store.Job{j}, nil
}

func (m *mockStore) UpdateJobStatus(ctx context.Context, id string, status store.JobStatus, stage string) error {
	atomic.AddInt32(&m.updated, 1)
	return nil
}

func (m *mockStore) CompleteJob(id string, status store.JobStatus) error {
	return nil
}

func (m *mockStore) UpdateJobError(id string, errMsg string) error {
	return nil
}

func TestPoolStartsAndStops(t *testing.T) {
	ms := &mockStore{}
	p := NewPool(3, ms, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := p.Start(ctx)
	require.NoError(t, err)
}

func TestPoolProcessesJob(t *testing.T) {
	ms := &mockStore{
		jobs: []store.Job{
			{ID: "j1", RepoFullName: "o/r", IssueNumber: 1, Status: store.JobQueued},
		},
	}

	var processed int32
	handler := func(ctx context.Context, job store.Job) error {
		atomic.AddInt32(&processed, 1)
		return nil
	}

	p := NewPool(1, ms, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.Start(ctx)
	time.Sleep(3 * time.Second)

	assert.Equal(t, int32(1), atomic.LoadInt32(&processed))
}

func TestPoolStopsOnContextCancel(t *testing.T) {
	ms := &mockStore{
		jobs: []store.Job{
			{ID: "j1", RepoFullName: "o/r", IssueNumber: 1, Status: store.JobQueued},
			{ID: "j2", RepoFullName: "o/r", IssueNumber: 2, Status: store.JobQueued},
		},
	}

	handler := func(ctx context.Context, job store.Job) error {
		<-ctx.Done()
		return ctx.Err()
	}

	p := NewPool(1, ms, handler)
	ctx, cancel := context.WithCancel(context.Background())

	p.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Pool should stop promptly — if goroutine leaks, test will timeout
	time.Sleep(500 * time.Millisecond)
}
