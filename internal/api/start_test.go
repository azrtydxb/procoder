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

// Exactly one caller wins when they all arrive at once.
func TestStartLockUnderRace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.lock")
	var mu sync.Mutex
	won := 0
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if release, taken := takeStartLock(path); taken {
				mu.Lock()
				won++
				mu.Unlock()
				time.Sleep(5 * time.Millisecond)
				release()
			}
		}()
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("%d callers won the start race, want exactly 1", won)
	}
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

// A half-written lock is one nobody is holding: the process died between
// creating the file and writing its pid.
func TestHalfWrittenStartLockIsStale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.lock")
	if err := os.WriteFile(path, []byte("12345\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !staleStartLock(path) {
		t.Fatal("a lock with no timestamp was treated as live")
	}
}

// listening tells a socket with a daemon behind it from the file a dead
// one left.
func TestListeningIgnoresADeadSocket(t *testing.T) {
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
