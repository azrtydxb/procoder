package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// Job states. Lost is deliberately not failed: a daemon that restarted
// does not know how the command got on, and saying it failed would be
// inventing an answer to a question nobody can answer any more.
const (
	JobRunning = "running"
	JobDone    = "done"
	JobLost    = "lost"
)

// longRunning is the commands that outlive a request, and the flag that
// makes some of them long.
//
// A list rather than a timeout: which commands run a whole toolchain over
// a whole tree is a fact about procoder, known here, and discovering it by
// waiting means every caller pays the wait once to find out.
var longRunning = map[string]string{
	"test":     "", // the whole suite, every ecosystem
	"audit":    "", // every domain over the whole tracked tree
	"release":  "", // version sync, changelog, the gate and the suite
	"bench":    "", // the benchmarks, against the saved baseline
	"deps":     "", // each ecosystem's native tool, most of them networked
	"security": "--deep",
	"docs":     "--external",
	"ci":       "--runs",
	"index":    "build",
}

// IsLongRunning reports whether an argv should answer with a job rather
// than a result.
func IsLongRunning(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	trigger, ok := longRunning[argv[0]]
	if !ok {
		return false
	}
	if trigger == "" {
		return true
	}
	for _, a := range argv[1:] {
		if a == trigger {
			return true
		}
	}
	return false
}

// jobState is one running command's accumulating answer.
type jobState struct {
	mu     sync.Mutex
	stdout bytes.Buffer
	stderr bytes.Buffer
	exit   *int
	result *Result
	state  string
}

// syncWriter lets the command write while a poller reads.
type syncWriter struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (w syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

// jobs is the daemon's job table. In memory only: a job that outlived the
// daemon would have to be re-attached to a process that no longer exists,
// and a caller whose daemon died needs to hear that rather than read a
// stale result.
type jobs struct {
	mu sync.Mutex
	m  map[string]*jobState
}

// start runs req in the background and returns the job to poll.
func (j *jobs) start(req Request, run Runner) Job {
	id := newJobID()
	st := &jobState{state: JobRunning}

	j.mu.Lock()
	if j.m == nil {
		j.m = map[string]*jobState{}
	}
	j.m[id] = st
	j.mu.Unlock()

	go func() {
		code, result := run(req,
			syncWriter{mu: &st.mu, buf: &st.stdout},
			syncWriter{mu: &st.mu, buf: &st.stderr})
		st.mu.Lock()
		st.exit = &code
		st.result = result
		st.state = JobDone
		st.mu.Unlock()
	}()

	return Job{ID: id, State: JobRunning}
}

// poll returns what a job has produced so far, and its exit code once
// there is one.
//
// The output accumulated so far comes back on every poll, so a caller can
// follow a suite without holding the connection that started it.
func (j *jobs) poll(id string, since, sinceErr int) Response {
	j.mu.Lock()
	st, ok := j.m[id]
	j.mu.Unlock()
	if !ok {
		return Response{Protocol: Protocol, Job: &Job{ID: id, State: JobLost}}
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	return Response{
		Protocol: Protocol,
		Exit:     st.exit,
		Stdout:   after(st.stdout.String(), since),
		Stderr:   after(st.stderr.String(), sinceErr),
		Job:      &Job{ID: id, State: st.state},
		Result:   st.result,
	}
}

// after is the part of s the caller does not have yet.
//
// An offset past the end returns nothing rather than panicking: a client
// that has more than the daemon does is a daemon that restarted, and the
// answer to that is an empty read and a lost job, never a crash in the
// one process everything else depends on.
func after(s string, n int) string {
	if n <= 0 {
		return s
	}
	if n >= len(s) {
		return ""
	}
	return s[n:]
}

// newJobID is short enough to type and random enough not to collide
// within one daemon's lifetime, which is the only scope it has.
func newJobID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A daemon that cannot read randomness still has to answer. The
		// address of the slice is unique among live jobs, which is the
		// only property an id needs here.
		return "j" + hex.EncodeToString([]byte{byte(len(b))})
	}
	return "j" + hex.EncodeToString(b[:])
}
