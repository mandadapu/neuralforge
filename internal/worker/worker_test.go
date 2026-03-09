package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
)

// trackingStore wraps the pool_test.go mockStore with atomic tracking for worker tests.
type trackingStore struct {
	store.Store
	updateErrorCalled atomic.Bool
	completeJobCalled atomic.Bool
}

func (m *trackingStore) UpdateJobError(_ context.Context, _ string, _ string) error {
	m.updateErrorCalled.Store(true)
	return nil
}

func (m *trackingStore) CompleteJob(_ context.Context, _ string, _ store.JobStatus) error {
	m.completeJobCalled.Store(true)
	return nil
}

func TestWorkerRunSuccess(t *testing.T) {
	ts := &trackingStore{}
	handlerCalled := false

	handler := func(_ context.Context, _ store.Job) error {
		handlerCalled = true
		return nil
	}

	w := &Worker{id: 0, handler: handler, store: ts}
	jobs := make(chan store.Job, 1)
	jobs <- store.Job{ID: "job-1", RepoFullName: "owner/repo", IssueNumber: 1}
	close(jobs)

	w.Run(context.Background(), jobs)

	assert.True(t, handlerCalled)
	assert.True(t, ts.completeJobCalled.Load())
	assert.False(t, ts.updateErrorCalled.Load())
}

func TestWorkerRunHandlerError(t *testing.T) {
	ts := &trackingStore{}

	handler := func(_ context.Context, _ store.Job) error {
		return errors.New("pipeline failed")
	}

	w := &Worker{id: 0, handler: handler, store: ts}
	jobs := make(chan store.Job, 1)
	jobs <- store.Job{ID: "job-2", RepoFullName: "owner/repo", IssueNumber: 2}
	close(jobs)

	w.Run(context.Background(), jobs)

	assert.True(t, ts.updateErrorCalled.Load())
	assert.False(t, ts.completeJobCalled.Load())
}

func TestWorkerRunContextCancelled(t *testing.T) {
	ts := &trackingStore{}
	handlerCalled := false

	handler := func(_ context.Context, _ store.Job) error {
		handlerCalled = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	w := &Worker{id: 0, handler: handler, store: ts}
	// Channel is closed without any jobs, simulating pool shutdown.
	jobs := make(chan store.Job)
	close(jobs)

	w.Run(ctx, jobs)

	assert.False(t, handlerCalled)
}

func TestWorkerRunMultipleJobs(t *testing.T) {
	ts := &trackingStore{}
	var handledCount atomic.Int32

	handler := func(_ context.Context, _ store.Job) error {
		handledCount.Add(1)
		return nil
	}

	w := &Worker{id: 0, handler: handler, store: ts}
	jobs := make(chan store.Job, 3)
	jobs <- store.Job{ID: "job-1"}
	jobs <- store.Job{ID: "job-2"}
	jobs <- store.Job{ID: "job-3"}
	close(jobs)

	w.Run(context.Background(), jobs)

	assert.Equal(t, int32(3), handledCount.Load())
}
