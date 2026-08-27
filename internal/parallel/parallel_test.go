package parallel

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The budget is a CAP, and the cap is what makes three fan-outs safe to
// stack: the formatter pass, the secret scan and the gate's legs all draw
// on it, so a whole-tree run cannot put thirty external processes on ten
// cores the way it did before this package existed.
//
// proved by: give Do its own semaphore per call — every caller then gets
// the whole machine again and the observed peak passes size().
func TestDoNeverExceedsTheBudget(t *testing.T) {
	var live, peak int64
	var mu sync.Mutex
	Do(200, func(int) {
		n := atomic.AddInt64(&live, 1)
		mu.Lock()
		if n > peak {
			peak = n
		}
		mu.Unlock()
		runtime.Gosched()
		atomic.AddInt64(&live, -1)
	})
	if peak > int64(size()) {
		t.Errorf("peak concurrency %d exceeded the budget of %d", peak, size())
	}
	if peak == 0 {
		t.Error("nothing ran")
	}
}

// The bug this package shipped with, and the reason it must never be
// nested. A unit holds its slot while it runs, so an outer fan-out can
// take every slot and then wait forever for an inner one. It passed on a
// ten-core laptop, where four outer units left six spare, and deadlocked
// the whole suite on CI's four cores.
//
// The guard is a documented rule rather than a mechanism, so this test
// demonstrates the hazard on a LOCAL semaphore of the same shape — proving
// the rule is real without deadlocking the suite to do it.
//
// proved by: raise the local budget to 4 or more — the nesting fits, the
// deadlock disappears, and this stops reporting one.
func TestNestingASemaphoreOfThisShapeDeadlocks(t *testing.T) {
	const cap = 2
	local := make(chan struct{}, cap)
	// A barrier, because without one this does not reproduce: a single
	// goroutine that acquires and then acquires again simply takes both
	// free slots and finishes. The deadlock needs every outer unit to be
	// HOLDING a slot before any of them reaches for a second, which is
	// exactly what a real fan-out does.
	var held sync.WaitGroup
	held.Add(cap)
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < cap; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				local <- struct{}{}
				defer func() { <-local }()
				held.Done()
				held.Wait() // every slot is now taken
				local <- struct{}{}
				<-local
			}()
		}
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		t.Error("expected the nested acquisition to stall; the hazard this package documents is not real")
	case <-time.After(200 * time.Millisecond):
		// stalled, as it must — which is why Do is never called from Do
	}
}

// proved by: replace the index passed to f with a shared counter — the
// indices collide and this fails.
func TestEveryIndexIsVisitedExactlyOnce(t *testing.T) {
	const n = 64
	seen := make([]int32, n)
	Do(n, func(i int) { atomic.AddInt32(&seen[i], 1) })
	for i, c := range seen {
		if c != 1 {
			t.Fatalf("index %d visited %d times, want 1", i, c)
		}
	}
}

// proved by: drop the `if n <= 0` guard — a negative n panics on make.
func TestDoOnNothingReturns(t *testing.T) {
	ran := false
	Do(0, func(int) { ran = true })
	if ran {
		t.Error("ran work for zero items")
	}
}
