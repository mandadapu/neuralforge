package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mandadapu/neuralforge/internal/store"
)

type JobHandler func(ctx context.Context, job store.Job) error

type Pool struct {
	size    int
	store   store.Store
	handler JobHandler
	jobs    chan store.Job
}

func NewPool(size int, s store.Store, handler JobHandler) *Pool {
	return &Pool{
		size:    size,
		store:   s,
		handler: handler,
		jobs:    make(chan store.Job, size*2),
	}
}

func (p *Pool) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	for i := 0; i < p.size; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w := &Worker{id: id, handler: p.handler, store: p.store}
			w.Run(ctx, p.jobs)
		}(i)
	}

	go p.poll(ctx)

	go func() {
		<-ctx.Done()
		close(p.jobs)
		wg.Wait()
	}()

	return nil
}

func (p *Pool) poll(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := p.store.ClaimPendingJobs(p.size)
			if err != nil {
				slog.Error("poll error", "error", err)
				continue
			}
			for _, job := range jobs {
				select {
				case p.jobs <- job:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
