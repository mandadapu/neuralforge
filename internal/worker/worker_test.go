package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
)

// workerMockStore is a separate mock from the one in pool_test.go,
// tracking UpdateJobError and CompleteJob calls.
type workerMockStore struct {
	store.Store
	updateCalled   bool
	completeCalled bool
}

func (m *workerMockStore) UpdateJobError(_ context.Context, _ string, _ string) error {
	m.updateCalled = true
	return nil
}

func (m *workerMockStore) CompleteJob(_ context.Context, _ string, _ store.JobStatus) error {
	m.completeCalled = true
	return nil
}

func TestWorker_RunSuccess(t *testing.T) {
	ms := &workerMockStore{}
	jobs := make(chan store.Job, 1)
	jobs <- store.Job{ID: "j1"}
	close(jobs)

	handler := func(_ context.Context, _ store.Job) error { return nil }
	w := &Worker{id: 1, handler: handler, store: ms}
	w.Run(context.Background(), jobs)

	assert.True(t, ms.completeCalled)
	assert.False(t, ms.updateCalled)
}

func TestWorker_RunJobFailure(t *testing.T) {
	ms := &workerMockStore{}
	jobs := make(chan store.Job, 1)
	jobs <- store.Job{ID: "j1"}
	close(jobs)

	handler := func(_ context.Context, _ store.Job) error { return errors.New("boom") }
	w := &Worker{id: 1, handler: handler, store: ms}
	w.Run(context.Background(), jobs)

	assert.True(t, ms.updateCalled)
	assert.False(t, ms.completeCalled)
}

func TestWorker_RunExitsOnContextCancellation(t *testing.T) {
	ms := &workerMockStore{}
	jobs := make(chan store.Job) // never sends a job

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		w := &Worker{id: 1, handler: func(_ context.Context, _ store.Job) error { return nil }, store: ms}
		w.Run(ctx, jobs)
		close(done)
	}()

	cancel()
	close(jobs) // unblock range

	select {
	case <-done:
		// Run returned promptly
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after context cancel and channel close")
	}
}

func TestWorker_RunMultipleJobs(t *testing.T) {
	ms := &workerMockStore{}
	jobs := make(chan store.Job, 3)
	jobs <- store.Job{ID: "j1"}
	jobs <- store.Job{ID: "j2"}
	jobs <- store.Job{ID: "j3"}
	close(jobs)

	var processed int
	handler := func(_ context.Context, _ store.Job) error {
		processed++
		return nil
	}
	w := &Worker{id: 1, handler: handler, store: ms}
	w.Run(context.Background(), jobs)

	assert.Equal(t, 3, processed)
	assert.True(t, ms.completeCalled)
}
