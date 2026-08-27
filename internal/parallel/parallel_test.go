package parallel

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// The whole point: nested fan-outs draw on ONE budget. Three of them each
// sized NumCPU is what put twenty to thirty external processes on ten
// cores, made the gate slower than any single change had made it faster,
// and starved the mermaid checker into a ninety-second timeout that
// reported two diagrams as NOT checked.
//
// proved by: give Do its own semaphore per call instead of the package
// budget — the outer and inner fan-outs stop sharing, peak concurrency
// passes the cap, and this fails.
func TestNestedFanOutsShareOneBudget(t *testing.T) {
	var live, peak int64
	var mu sync.Mutex
	work := func() {
		n := atomic.AddInt64(&live, 1)
		mu.Lock()
		if n > peak {
			peak = n
		}
		mu.Unlock()
		runtime.Gosched()
		atomic.AddInt64(&live, -1)
	}
	// An outer fan-out whose every unit opens an inner one, which is the
	// shape the gate has: legs, and files inside a leg.
	Do(4, func(int) { Do(8, func(int) { work() }) })

	cap := int64(maxInt(1, runtime.NumCPU()))
	if peak > cap {
		t.Errorf("peak concurrency %d exceeded the budget of %d", peak, cap)
	}
	if peak == 0 {
		t.Error("nothing ran")
	}
}

// proved by: drop the `if n <= 0` guard — Do then builds a zero-length
// channel and the receive loop returns immediately, which is harmless, but
// a negative n panics on make. This pins the boundary either way.
func TestDoOnNothingReturns(t *testing.T) {
	ran := false
	Do(0, func(int) { ran = true })
	if ran {
		t.Error("ran work for zero items")
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
