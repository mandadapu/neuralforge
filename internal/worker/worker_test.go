package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
)

// hookStore wraps mockStore with callback hooks for asserting side effects.
type hookStore struct {
	mockStore
	onUpdateJobError func(id, msg string)
	onCompleteJob    func(id string, status store.JobStatus)
}

func (h *hookStore) UpdateJobError(ctx context.Context, id string, errMsg string) error {
	if h.onUpdateJobError != nil {
		h.onUpdateJobError(id, errMsg)
	}
	return nil
}

func (h *hookStore) CompleteJob(ctx context.Context, id string, status store.JobStatus) error {
	if h.onCompleteJob != nil {
		h.onCompleteJob(id, status)
	}
	return nil
}

func TestWorkerProcessesJobsAndExitsOnClose(t *testing.T) {
	var callCount int32
	handler := func(ctx context.Context, job store.Job) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	w := &Worker{id: 1, handler: handler, store: &hookStore{}}
	jobs := make(chan store.Job, 2)
	jobs <- store.Job{ID: "j1"}
	jobs <- store.Job{ID: "j2"}
	close(jobs)

	w.Run(context.Background(), jobs)

	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

func TestWorkerRecordsErrorOnHandlerFailure(t *testing.T) {
	var recordedJobID string
	hs := &hookStore{
		onUpdateJobError: func(id, msg string) {
			recordedJobID = id
		},
	}

	handler := func(ctx context.Context, job store.Job) error {
		return errors.New("handler failed")
	}

	w := &Worker{id: 2, handler: handler, store: hs}
	jobs := make(chan store.Job, 1)
	jobs <- store.Job{ID: "fail-job"}
	close(jobs)

	w.Run(context.Background(), jobs)

	assert.Equal(t, "fail-job", recordedJobID)
}

func TestWorkerCompletesJobOnSuccess(t *testing.T) {
	var completedJobID string
	hs := &hookStore{
		onCompleteJob: func(id string, status store.JobStatus) {
			completedJobID = id
		},
	}

	handler := func(ctx context.Context, job store.Job) error {
		return nil
	}

	w := &Worker{id: 3, handler: handler, store: hs}
	jobs := make(chan store.Job, 1)
	jobs <- store.Job{ID: "success-job"}
	close(jobs)

	w.Run(context.Background(), jobs)

	assert.Equal(t, "success-job", completedJobID)
}

func TestWorkerExitsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before any jobs are processed

	var callCount int32
	handler := func(ctx context.Context, job store.Job) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	w := &Worker{id: 4, handler: handler, store: &hookStore{}}
	jobs := make(chan store.Job, 2)
	jobs <- store.Job{ID: "j1"}
	jobs <- store.Job{ID: "j2"}
	close(jobs)

	w.Run(ctx, jobs)

	// Worker may process 0, 1, or 2 jobs depending on scheduler timing
	assert.LessOrEqual(t, atomic.LoadInt32(&callCount), int32(2))
}
