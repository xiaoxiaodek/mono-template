package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunWorkersBoundsConcurrency(t *testing.T) {
	jobs := make(chan Job, 8)
	var active atomic.Int32
	var peak atomic.Int32
	for range 8 {
		jobs <- func(context.Context) error {
			current := active.Add(1)
			for {
				observed := peak.Load()
				if current <= observed || peak.CompareAndSwap(observed, current) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			active.Add(-1)
			return nil
		}
	}
	close(jobs)

	if err := RunWorkers(context.Background(), 2, jobs); err != nil {
		t.Fatalf("RunWorkers: %v", err)
	}
	if got := peak.Load(); got > 2 {
		t.Fatalf("peak concurrency = %d, want at most 2", got)
	}
}

func TestRunWorkersRejectsUnboundedWorkerCount(t *testing.T) {
	jobs := make(chan Job)
	close(jobs)
	if err := RunWorkers(context.Background(), MaxWorkers+1, jobs); err == nil {
		t.Fatal("RunWorkers accepted a worker count above MaxWorkers")
	}
}
