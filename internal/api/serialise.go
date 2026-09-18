package api

import "sync"

// queues serialises requests per repository.
//
// This is not an optimisation, it is a correctness fix, and the reason is
// in internal/store: the lock there is an O_EXCL lockfile with an mtime
// heartbeat and no in-process registry. Two goroutines of ONE daemon
// hitting the same repository do not queue behind each other — the second
// spins, finds a lock the first one's heartbeat keeps fresh so breakStale
// never fires, reaches lockTimeout, and returns "the write was NOT made".
//
// Today that is rare because every hook is its own process arriving at its
// own moment. A daemon makes concurrent same-repository work ordinary, so
// without this the change converts a rare race into a routine five-second
// failure — the ledgers most exposed being the read-modify-write ones:
// dispatch, claims, and the ask queue.
//
// Per repository rather than one global lock: two sessions in two
// checkouts share nothing, and serialising them would make the daemon
// slower than the spawning it replaced.
type queues struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

// do runs f with the repository's queue held.
//
// An empty identity gets its own queue rather than sharing one with every
// other unidentifiable request: an unknown repository is not the same
// repository as another unknown one.
func (q *queues) do(identity string, f func()) {
	q.mu.Lock()
	if q.m == nil {
		q.m = map[string]*sync.Mutex{}
	}
	lock, ok := q.m[identity]
	if !ok {
		lock = &sync.Mutex{}
		q.m[identity] = lock
	}
	q.mu.Unlock()

	lock.Lock()
	defer lock.Unlock()
	f()
}
