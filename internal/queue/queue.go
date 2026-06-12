// Package queue provides a thread-safe, deduplicating priority queue of
// bug.Bug records for the scheduler → resolver handoff.
//
// Higher Priority scores are served first. Bugs already in the queue are
// silently dropped on a second Push with the same ID.
package queue

import (
	"container/heap"
	"sync"

	"github.com/nravada/nr-smash/internal/bug"
)

// Queue is a thread-safe priority queue of Bug records.
type Queue struct {
	mu   sync.Mutex
	h    bugHeap
	seen map[string]struct{}
}

// New returns an empty, ready-to-use Queue.
func New() *Queue {
	q := &Queue{seen: make(map[string]struct{})}
	heap.Init(&q.h)
	return q
}

// Push adds b to the queue. If a bug with the same ID is already present
// the call is a no-op.
func (q *Queue) Push(b *bug.Bug) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, dup := q.seen[b.ID]; dup {
		return
	}
	q.seen[b.ID] = struct{}{}
	heap.Push(&q.h, b)
}

// Pop removes and returns the highest-priority bug. Returns nil if empty.
func (q *Queue) Pop() *bug.Bug {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.h.Len() == 0 {
		return nil
	}
	b := heap.Pop(&q.h).(*bug.Bug)
	delete(q.seen, b.ID)
	return b
}

// Peek returns the highest-priority bug without removing it. Returns nil
// if empty.
func (q *Queue) Peek() *bug.Bug {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.h.Len() == 0 {
		return nil
	}
	return q.h[0]
}

// Len returns the number of bugs currently queued.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.h.Len()
}

// Snapshot returns a copy of all queued bugs in arbitrary order.
func (q *Queue) Snapshot() []*bug.Bug {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*bug.Bug, len(q.h))
	copy(out, q.h)
	return out
}

// bugHeap implements heap.Interface for *bug.Bug (max-heap by Priority).
type bugHeap []*bug.Bug

func (h bugHeap) Len() int           { return len(h) }
func (h bugHeap) Less(i, j int) bool { return h[i].Priority > h[j].Priority } // max-heap
func (h bugHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *bugHeap) Push(x any) { *h = append(*h, x.(*bug.Bug)) }

func (h *bugHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}
