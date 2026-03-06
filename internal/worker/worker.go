package worker

import (
	"context"
	"log/slog"

	"github.com/mandadapu/neuralforge/internal/store"
)

type Worker struct {
	id      int
	handler JobHandler
	store   store.Store
}

func (w *Worker) Run(ctx context.Context, jobs <-chan store.Job) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		slog.Info("worker processing job", "worker", w.id, "job", job.ID)

		if err := w.handler(ctx, job); err != nil {
			slog.Error("job failed", "worker", w.id, "job", job.ID, "error", err)
			_ = w.store.UpdateJobError(ctx, job.ID, err.Error())
			continue
		}

		_ = w.store.CompleteJob(ctx, job.ID, store.JobCompleted)
		slog.Info("job completed", "worker", w.id, "job", job.ID)
	}
}
