package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

const MaxWorkers = 256

// Job is one context-aware unit of work.
type Job func(context.Context) error

// RunWorkers consumes jobs with a fixed number of goroutines and waits for
// all workers to finish. The first job error cancels the remaining workers.
func RunWorkers(ctx context.Context, workerCount int, jobs <-chan Job) error {
	if workerCount <= 0 || workerCount > MaxWorkers {
		return fmt.Errorf("worker count must be between 1 and %d", MaxWorkers)
	}

	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var workers sync.WaitGroup
	var firstErr error
	var errorOnce sync.Once
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for {
				select {
				case <-workerCtx.Done():
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					if job == nil {
						continue
					}
					if err := job(workerCtx); err != nil {
						errorOnce.Do(func() {
							firstErr = err
							cancel()
						})
						return
					}
				}
			}
		}()
	}
	workers.Wait()

	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return ctx.Err()
}
