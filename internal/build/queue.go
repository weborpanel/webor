package build

import (
	"context"
	"sync"
	"sync/atomic"
)

// Queue serializes builds so only one runs at a time on the server.
type Queue struct {
	once    sync.Once
	ch      chan struct{}
	waiting atomic.Int32
}

func (q *Queue) init() {
	q.once.Do(func() {
		q.ch = make(chan struct{}, 1)
		q.ch <- struct{}{}
	})
}

// TryAcquire returns false immediately when another build holds the lock.
func (q *Queue) TryAcquire() bool {
	q.init()
	select {
	case <-q.ch:
		return true
	default:
		return false
	}
}

// Acquire waits until the global build slot is free or ctx is cancelled.
func (q *Queue) Acquire(ctx context.Context) error {
	q.init()
	q.waiting.Add(1)
	defer q.waiting.Add(-1)
	select {
	case <-q.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees the global build slot.
func (q *Queue) Release() {
	q.init()
	select {
	case q.ch <- struct{}{}:
	default:
		// already free
	}
}

// Stats returns whether a build is running and how many waiters are queued.
func (q *Queue) Stats() (busy bool, waiting int) {
	q.init()
	busy = len(q.ch) == 0
	return busy, int(q.waiting.Load())
}

// Global is the process-wide build queue.
var Global = &Queue{}
