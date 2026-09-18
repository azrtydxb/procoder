package api

import (
	"sync"
	"time"
)

// DefaultIdle is how long a repository's warm state outlives its last
// request.
//
// Thirty minutes because the thing being kept warm is a code index, and
// the gap between two pieces of work in one repository is a coffee, not a
// day. A window short enough to expire between two commits would throw
// away the index that is most of the reason to run a daemon.
const DefaultIdle = 30 * time.Minute

// repoState is what the daemon remembers about one repository.
//
// Per repository, evicted per repository. A daemon serving ten checkouts
// holds ten of these and lets each one go on its own schedule, so a
// morning in one repository does not keep nine others' indexes resident.
type repoState struct {
	// Index is the repository's code index, held in memory rather than
	// re-read from disk on every request. It is an any because building it
	// belongs to internal/codeindex and the transport has no business
	// knowing its shape.
	Index any
	// Config is the repository's parsed configuration.
	Config any

	used time.Time
}

// warm is the daemon's per-repository memory.
type warm struct {
	mu     sync.Mutex
	m      map[string]*repoState
	window time.Duration
	// now is time.Now except in a test, which cannot wait out a real
	// thirty-minute window and must not be made to sleep for one.
	now func() time.Time
}

func (w *warm) clock() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

func (w *warm) idle() time.Duration {
	if w.window > 0 {
		return w.window
	}
	return DefaultIdle
}

// get returns the repository's state, creating it on first use and
// marking it used either way.
func (w *warm) get(identity string) *repoState {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.m == nil {
		w.m = map[string]*repoState{}
	}
	st, ok := w.m[identity]
	if !ok {
		st = &repoState{}
		w.m[identity] = st
	}
	st.used = w.clock()
	return st
}

// evict drops every repository whose window has passed and reports how
// many are still held.
//
// The count is the return value because it is what the daemon's own
// lifetime turns on: a daemon holding nothing has nothing to be warm for,
// and staying resident to serve a request that may never come is how a
// convenience becomes something a person has to remember to kill.
func (w *warm) evict(now time.Time) (held int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, st := range w.m {
		if now.Sub(st.used) >= w.idle() {
			delete(w.m, id)
			continue
		}
		held++
	}
	return held
}
