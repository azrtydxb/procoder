package api

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// takeStartLock is what makes ten sessions starting at once leave one
// daemon. Held by one caller at a time, and never held forever by a
// process that died taking it.
//
// proved by: dropping the O_EXCL — every caller then "takes" the lock and
// ten sessions start ten daemons, nine of which lose the socket race and
// exit having built an index nobody will use.
func TestStartLockIsHeldByOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.lock")

	release, taken := takeStartLock(path)
	if !taken {
		t.Fatal("the first caller did not get the lock")
	}
	if _, second := takeStartLock(path); second {
		t.Fatal("two callers hold the start lock — they will start two daemons")
	}
	release()
	release2, third := takeStartLock(path)
	if !third {
		t.Fatal("the lock was not released")
	}
	release2()
}

// Exactly one caller holds the lock at a time when they all arrive at
// once.
//
// Every winner holds until all ten have tried. The first version of this
// released after 5ms inside the goroutine, which measured something else
// entirely: a caller scheduled after that release acquires the lock
// legitimately, so "two callers won" was two callers winning in
// SEQUENCE — correct behaviour, counted as a failure. Windows found it,
// because its scheduling spreads ten goroutines over more than the five
// milliseconds the winner was holding.
//
// What the lock promises is mutual exclusion, not that only one caller
// ever succeeds across time. This asserts the promise.
//
// proved by: dropping the O_EXCL in takeStartLock — several callers hold
// at once and the count goes above one.
func TestStartLockUnderRace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.lock")
	var mu sync.Mutex
	held := 0
	var releases []func()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if release, taken := takeStartLock(path); taken {
				mu.Lock()
				held++
				releases = append(releases, release)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if held != 1 {
		t.Fatalf("%d callers held the start lock at once, want exactly 1", held)
	}
	for _, release := range releases {
		release()
	}

	// And the lock is reusable once released: the next caller gets it.
	release, taken := takeStartLock(path)
	if !taken {
		t.Fatal("the lock was not released")
	}
	release()
}

// A lock left behind by a process that died mid-start does not block
// every session after it forever.
func TestStaleStartLockIsBroken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.lock")
	old := time.Now().Add(-2 * startStale).Unix()
	if err := os.WriteFile(path, []byte("999999\n"+strconv.FormatInt(old, 10)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	release, taken := takeStartLock(path)
	if !taken {
		t.Fatal("a stale start lock blocked a new start")
	}
	release()
}

// A lock taken a moment ago is live and must not be broken.
func TestFreshStartLockIsNotBroken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.lock")
	release, taken := takeStartLock(path)
	if !taken {
		t.Fatal(taken)
	}
	defer release()
	if _, second := takeStartLock(path); second {
		t.Fatal("a live start lock was broken — two daemons will start")
	}
}

// A lock with no timestamp yet is judged by its age, because that is the
// only thing left that can tell the two cases apart: a process that died
// between creating the file and writing to it, and the winner of the race
// that has not finished writing YET.
//
// Calling it stale outright was a bug with a name. Ten sessions starting
// at once had three of them win: the losers read the winner's empty file,
// judged it abandoned, removed it and took their own — three daemons
// where one was meant to be. It failed about one run in ten.
//
// proved by: returning true for a short lock regardless of age — the race
// test below starts finding two and three winners again.
func TestHalfWrittenStartLockIsJudgedByAge(t *testing.T) {
	dir := t.TempDir()

	fresh := filepath.Join(dir, "fresh.lock")
	if err := os.WriteFile(fresh, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if staleStartLock(fresh) {
		t.Error("a lock written moments ago was judged abandoned — that is the winner of the race, mid-write")
	}

	old := filepath.Join(dir, "old.lock")
	if err := os.WriteFile(old, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * startStale)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if !staleStartLock(old) {
		t.Error("a half-written lock older than the stale window was treated as live — nothing would ever start")
	}
}

// listening tells a socket with a daemon behind it from the file a dead
// one left.
func TestListeningIgnoresADeadSocket(t *testing.T) {
	requireDaemon(t)
	path := filepath.Join(shortDir(t), "s.sock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if listening(path) {
		t.Fatal("a socket file with nothing behind it was reported as a daemon")
	}

	live, _ := testServer(t, nil)
	if !listening(live) {
		t.Fatal("a live daemon was not reported as listening")
	}
}
