package api

import (
	"io"
	"path/filepath"
	"testing"
	"time"
)

// Each repository's warm state expires on its own schedule. A morning in
// one repository must not keep nine others' indexes resident.
//
// proved by: evicting everything once the oldest entry expires — the
// repository somebody is still working in loses its index too, and the
// next request rebuilds what was already there.
func TestPerRepoEviction(t *testing.T) {
	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	now := base
	w := &warm{window: 10 * time.Minute, now: func() time.Time { return now }}

	w.get("old")
	now = base.Add(9 * time.Minute)
	w.get("fresh")

	// Eleven minutes after "old" was used and two after "fresh" was.
	if held := w.evict(base.Add(11 * time.Minute)); held != 1 {
		t.Fatalf("want 1 repository still held, got %d", held)
	}
	if _, ok := w.m["old"]; ok {
		t.Error("a repository past its window is still held")
	}
	if _, ok := w.m["fresh"]; !ok {
		t.Error("a repository inside its window was evicted")
	}
}

// Using a repository again restarts its window: work in progress does not
// expire because it started thirty minutes ago.
func TestUseKeepsARepositoryWarm(t *testing.T) {
	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	now := base
	w := &warm{window: 10 * time.Minute, now: func() time.Time { return now }}

	w.get("busy")
	now = base.Add(9 * time.Minute)
	w.get("busy")

	if held := w.evict(base.Add(11 * time.Minute)); held != 1 {
		t.Fatalf("a repository used two minutes ago was evicted: held %d", held)
	}
}

// A daemon holding nothing says so, which is what its own lifetime turns
// on.
func TestEvictReportsNothingHeld(t *testing.T) {
	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	now := base
	w := &warm{window: time.Minute, now: func() time.Time { return now }}
	w.get("one")
	if held := w.evict(base.Add(2 * time.Minute)); held != 0 {
		t.Fatalf("want nothing held, got %d", held)
	}
}

// A daemon that has been holding nothing for a whole window stops.
//
// Exiting is safe because starting is free: the next session's hook starts
// another, and a client that finds no daemon runs in-process.
//
// proved by: never closing the listener in evictUntilEmpty — Accept then
// never returns, and the daemon outlives every session that used it as a
// process somebody has to know to kill.
func TestDaemonExitsHoldingNothing(t *testing.T) {
	path := filepath.Join(shortDir(t), "s.sock")
	srv := &Server{
		Run:    func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil },
		Notice: io.Discard,
		Idle:   60 * time.Millisecond,
	}
	l, err := srv.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	returned := make(chan struct{})
	go func() { srv.Accept(l); close(returned) }()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("a daemon holding nothing is still accepting")
	}
}

// A daemon with work in front of it does not exit out from under it.
func TestBusyDaemonStaysUp(t *testing.T) {
	path := filepath.Join(shortDir(t), "s.sock")
	srv := &Server{
		Run:      func(Request, io.Writer, io.Writer) (int, *Result) { return 0, nil },
		Notice:   io.Discard,
		Idle:     300 * time.Millisecond,
		Identity: func(string) string { return "one" },
	}
	l, err := srv.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	returned := make(chan struct{})
	go func() { srv.Accept(l); close(returned) }()

	// Keep it busy for longer than one window.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := (Client{Path: path}).Do(Request{Argv: []string{"config"}}); err != nil {
			t.Fatalf("the daemon stopped serving while it was being used: %v", err)
		}
		time.Sleep(40 * time.Millisecond)
	}

	select {
	case <-returned:
		t.Fatal("a daemon being used exited")
	default:
	}
}
