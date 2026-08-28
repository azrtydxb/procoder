package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// plant writes a lock file for rel with the given pid and unix time, so a
// test can stage the exact state the stale check has to judge.
func plant(t *testing.T, root, rel string, pid int, unix int64) {
	t.Helper()
	p := lockPath(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%d\n%d\n", pid, unix)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// proved by: dropping the O_EXCL flag makes the second Lock succeed.
func TestLockIsExclusive(t *testing.T) {
	root := t.TempDir()
	rel, _, err := Lock(root, ".procoder/state/dispatch.json")
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer rel()
	if _, _, err := Lock(root, ".procoder/state/dispatch.json"); err == nil {
		t.Fatal("second lock succeeded while the first was held")
	}
}

// proved by: removing the age check leaves the lock held and the write refused.
func TestStaleLockIsBrokenAndReported(t *testing.T) {
	root := t.TempDir()
	plant(t, root, ".procoder/state/dispatch.json", os.Getpid(), time.Now().Add(-31*time.Second).Unix())
	rel, broken, err := Lock(root, ".procoder/state/dispatch.json")
	if err != nil {
		t.Fatalf("stale lock was not broken: %v", err)
	}
	defer rel()
	if len(broken) != 1 || !strings.Contains(broken[0], "dispatch.json") {
		t.Fatalf("breaking the lock was not reported: %v", broken)
	}
}

// proved by: dropping the mtime clause or the future check leaves that case
// waiting out the full timeout instead of taking the lock.
//
// The unparsable case is planted with an OLD mtime deliberately. A lock file
// is created empty and written a moment later, so unparsable contents on a
// fresh file mean "being written now", not "dead" — treating those as dead
// gave two holders of one lock.
func TestUnreadableLockIsStale(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"unparsable", "not a lock\n"},
		{"future", fmt.Sprintf("%d\n%d\n", os.Getpid(), time.Now().Add(time.Hour).Unix())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := lockPath(root, ".procoder/state/dispatch.json")
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.name == "unparsable" {
				old := time.Now().Add(-31 * time.Second)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			}
			rel, broken, err := Lock(root, ".procoder/state/dispatch.json")
			if err != nil {
				t.Fatalf("lock was not treated as stale: %v", err)
			}
			defer rel()
			if len(broken) != 1 {
				t.Fatalf("breaking the lock was not reported: %v", broken)
			}
		})
	}
}

// proved by: removing the deadline from the retry loop makes this hang rather
// than fail, which is the failure a hook must never be able to cause.
func TestLiveLockRefusesRatherThanBlocks(t *testing.T) {
	root := t.TempDir()
	plant(t, root, ".procoder/state/dispatch.json", os.Getpid(), time.Now().Unix())
	start := time.Now()
	_, _, err := Lock(root, ".procoder/state/dispatch.json")
	if err == nil {
		t.Fatal("write proceeded while a live lock was held")
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Fatalf("Lock blocked for %v, past its 5s timeout", d)
	}
	if !strings.Contains(err.Error(), ".procoder/state/dispatch.json") {
		t.Fatalf("error does not name the file: %v", err)
	}
}

// proved by: locking in the order the caller asked, rather than sorted, makes
// the two goroutines deadlock and the timeout fire.
func TestLockOrderIsSortedPaths(t *testing.T) {
	root := t.TempDir()
	a, b := ".procoder/backlog/stories/s1.md", ".procoder/backlog/sprints/001.md"
	done := make(chan error, 2)
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		go func(p [2]string) {
			for i := 0; i < 50; i++ {
				rel, _, err := Lock(root, p[0], p[1])
				if err != nil {
					done <- err
					return
				}
				rel()
			}
			done <- nil
		}(pair)
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("lock failed: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("deadlock: locks were not taken in sorted order")
		}
	}
}

// proved by: returning early on the first failure, without releasing what was
// already taken, leaves the first path locked and this fails.
func TestPartialAcquisitionReleasesWhatItTook(t *testing.T) {
	root := t.TempDir()
	first, second := ".procoder/a.md", ".procoder/b.md"
	plant(t, root, second, os.Getpid(), time.Now().Unix()) // live, will not yield
	if _, _, err := Lock(root, first, second); err == nil {
		t.Fatal("Lock succeeded though the second path was held")
	}
	rel, _, err := Lock(root, first)
	if err != nil {
		t.Fatalf("the first path was left locked after a failed multi-lock: %v", err)
	}
	rel()
}

// proved by: treating unparsable contents as stale regardless of the file's
// age lets a caller steal a lock that was created a microsecond ago and is
// still being written. Two holders of one lock is the defect this package
// exists to remove, so it gets its own test rather than only showing up as a
// lost append somewhere downstream.
func TestNewbornLockIsNotStolen(t *testing.T) {
	root := t.TempDir()
	p := lockPath(root, ".procoder/state/dispatch.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	// exactly what O_EXCL leaves behind for the moment before the pid and
	// timestamp are written
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if stale(p) {
		t.Fatal("an empty, freshly created lock was judged stale — a live lock can be stolen mid-write")
	}
}
