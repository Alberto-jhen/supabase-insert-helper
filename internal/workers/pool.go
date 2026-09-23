package workers

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/Alberto-jhen/supabase-insert-helper/internal/models"
)

// Processor defines the function executed by each worker for a single job.
type Processor func(context.Context, models.ImageJob) models.ImageResult

// Pool distributes image jobs across a fixed number of workers using channels.
type Pool struct {
	workers   int
	jobs      chan models.ImageJob
	results   chan models.ImageResult
	processor Processor
	wg        sync.WaitGroup
	done      chan struct{}
	stopOnce  sync.Once
}

// NewPool creates a pool with the given number of workers and job processor.
func NewPool(workers int, processor Processor) *Pool {
	if workers <= 0 {
		workers = 1
	}
	return &Pool{
		workers:   workers,
		jobs:      make(chan models.ImageJob, workers*8),
		results:   make(chan models.ImageResult, workers*8),
		processor: processor,
		done:      make(chan struct{}),
	}
}

// Start launches the worker goroutines. They run until the pool is stopped.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.runWorker(ctx, i)
	}
	slog.Info("worker pool started", "workers", p.workers)
}

func (p *Pool) runWorker(ctx context.Context, id int) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down", "id", id, "reason", ctx.Err())
			return
		case job, ok := <-p.jobs:
			if !ok {
				slog.Info("worker finished", "id", id)
				return
			}
			result := p.processor(ctx, job)
			select {
			case p.results <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Submit sends a job to the worker pool. It respects context cancellation and
// returns an error if the pool has already been stopped.
func (p *Pool) Submit(ctx context.Context, job models.ImageJob) error {
	select {
	case p.jobs <- job:
		return nil
	case <-p.done:
		return errors.New("worker pool is stopped")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Results returns the read-only channel of job results.
func (p *Pool) Results() <-chan models.ImageResult {
	return p.results
}

// Stop closes the jobs channel, waits for workers to finish, and closes the
// results channel. It is safe to call multiple times.
func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		close(p.jobs)
		p.wg.Wait()
		close(p.results)
		close(p.done)
		slog.Info("worker pool stopped")
	})
}
