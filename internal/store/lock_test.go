package store

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// impatient shortens the acquire timeout for a test that is deliberately
// made to wait it out. Six tests each sleeping five real seconds is thirty
// seconds of a suite that otherwise runs in two, and the number under test
// is the behaviour at the deadline, not the deadline itself.
// TestMain silences the break notice for the whole package. Setting it
// per test raced the tests that restore it against anything still holding
// a lock, and `go test -race` said so.
func TestMain(m *testing.M) {
	Notice = io.Discard
	os.Exit(m.Run())
}

func impatient(t *testing.T) {
	t.Helper()
	was := lockTimeout
	lockTimeout = 50 * time.Millisecond
	t.Cleanup(func() { lockTimeout = was })
}

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
	// The file's mtime is the liveness signal, so a lock planted as having
	// been taken N seconds ago must LOOK N seconds old. Writing fresh bytes
	// with an old timestamp inside would be a lock no real run produces.
	at := time.Unix(unix, 0)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

// proved by: dropping the O_EXCL flag makes the second Lock succeed.
func TestLockIsExclusive(t *testing.T) {
	impatient(t)
	root := t.TempDir()
	rel, err := Lock(root, ".procoder/state/dispatch.json")
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer rel()
	if _, err := Lock(root, ".procoder/state/dispatch.json"); err == nil {
		t.Fatal("second lock succeeded while the first was held")
	}
}

// proved by: removing the age check leaves the lock held and the write refused.
func TestStaleLockIsBrokenAndReported(t *testing.T) {
	root := t.TempDir()
	plant(t, root, ".procoder/state/dispatch.json", os.Getpid(), time.Now().Add(-31*time.Second).Unix())
	var notice bytes.Buffer
	Notice = &notice
	t.Cleanup(func() { Notice = io.Discard })

	rel, err := Lock(root, ".procoder/state/dispatch.json")
	if err != nil {
		t.Fatalf("stale lock was not broken: %v", err)
	}
	defer rel()
	if !strings.Contains(notice.String(), "dispatch.json") {
		t.Fatalf("breaking the lock was not reported: %q", notice.String())
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
			rel, err := Lock(root, ".procoder/state/dispatch.json")
			if err != nil {
				t.Fatalf("lock was not treated as stale: %v", err)
			}
			defer rel()
		})
	}
}

// proved by: removing the deadline from the retry loop makes this hang rather
// than fail, which is the failure a hook must never be able to cause.
func TestLiveLockRefusesRatherThanBlocks(t *testing.T) {
	impatient(t)
	root := t.TempDir()
	plant(t, root, ".procoder/state/dispatch.json", os.Getpid(), time.Now().Unix())
	start := time.Now()
	_, err := Lock(root, ".procoder/state/dispatch.json")
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
				rel, err := Lock(root, p[0], p[1])
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
	impatient(t)
	root := t.TempDir()
	first, second := ".procoder/a.md", ".procoder/b.md"
	plant(t, root, second, os.Getpid(), time.Now().Unix()) // live, will not yield
	if _, err := Lock(root, first, second); err == nil {
		t.Fatal("Lock succeeded though the second path was held")
	}
	rel, err := Lock(root, first)
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

// brief shortens the staleness window so a test can outlive it.
//
// Three seconds, not sixty milliseconds. os.Chtimes stores a TRUNCATED
// time on HFS+, exFAT and several container overlay filesystems — one or
// two seconds of granularity — so a lock touched a moment ago can read as
// nearly a second old, and a sub-second window would fail there for a
// reason nobody changed. The six seconds of sleeping this was meant to
// save is in lockTimeout, which impatient() handles.
func brief(t *testing.T) {
	t.Helper()
	was := staleAfter
	staleAfter = 3 * time.Second
	t.Cleanup(func() { staleAfter = was })
}

// TestOnlyOneCallerEverHoldsALock is the test for the defect that review
// found and this package exists to prevent.
//
// Judging a lock stale and removing it are two operations. Without
// serialising them, two callers that both judge the same lock stale
// interleave: the first removes and re-creates, the second removes what
// the first just created and creates its own. Both then hold, and the
// second's write lands on top of the first's.
//
// proved by: replacing breakStale's O_EXCL break file with a plain
// stat-then-remove makes this fail with "2 callers held the lock at once".
func TestOnlyOneCallerEverHoldsALock(t *testing.T) {
	root := t.TempDir()
	const rel = ".procoder/state/dispatch.json"
	// A dead lock in the way, so every caller arrives at the break path
	// together rather than one of them simply winning O_EXCL.
	plant(t, root, rel, os.Getpid(), time.Now().Add(-31*time.Second).Unix())
	var holders int32
	var worst int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := Lock(root, rel)
			if err != nil {
				return // losing the race is fine; holding it together is not
			}
			n := atomic.AddInt32(&holders, 1)
			for {
				w := atomic.LoadInt32(&worst)
				if n <= w || atomic.CompareAndSwapInt32(&worst, w, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt32(&holders, -1)
			release()
		}()
	}
	wg.Wait()
	if worst > 1 {
		t.Fatalf("%d callers held the lock at once", worst)
	}
}

// proved by: without the heartbeat the mtime is set once at creation, so a
// write that legitimately runs longer than staleAfter has its lock taken
// out from under it while it is still writing.
func TestAHeldLockIsNotBrokenWhileItsOwnerWorks(t *testing.T) {
	brief(t)
	impatient(t)
	root := t.TempDir()
	const rel = ".procoder/state/claims.json"

	release, err := Lock(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	// Outlive the staleness window, the way a slow write does.
	time.Sleep(2 * staleAfter)

	if _, err := Lock(root, rel); err == nil {
		t.Fatal("a lock held by a live, working owner was broken as stale")
	}
}

// TestBreakingAStaleLockIsSerialised is the real proof for the defect
// review found; TestOnlyOneCallerEverHoldsALock above is a stress test and
// is honestly NOT proof, because the window it hunts is two syscalls wide
// and sixteen goroutines rarely land inside it.
//
// The defect: judging a lock stale and removing it are two operations.
// Two callers that both judge the same lock stale interleave — the first
// removes and re-creates, the second removes what the first just created —
// and both then hold. The fix is that only the holder of the break file
// may remove, so the second caller never reaches its remove.
//
// proved by: removing the break file's O_EXCL guard makes this test break
// the lock anyway and fail.
func TestBreakingAStaleLockIsSerialised(t *testing.T) {
	impatient(t)
	root := t.TempDir()
	const rel = ".procoder/state/dispatch.json"
	plant(t, root, rel, os.Getpid(), time.Now().Add(-31*time.Second).Unix())

	// Somebody else is already inside the break.
	bp := lockPath(root, rel) + breakSuffix
	if err := os.WriteFile(bp, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Lock(root, rel); err == nil {
		t.Fatal("broke a stale lock while another caller was already breaking it — two callers can hold")
	}

	// And once that caller is done, the stale lock is breakable again.
	if err := os.Remove(bp); err != nil {
		t.Fatal(err)
	}
	Notice = io.Discard
	t.Cleanup(func() { Notice = os.Stderr })
	release, err := Lock(root, rel)
	if err != nil {
		t.Fatalf("the stale lock was not breakable once the break file went: %v", err)
	}
	release()
}

// proved by: never clearing an orphaned break file wedges the path — no
// caller can ever break the stale lock behind it, and every write to that
// file fails until somebody deletes it by hand.
func TestAnOrphanedBreakFileIsCleared(t *testing.T) {
	root := t.TempDir()
	const rel = ".procoder/state/dispatch.json"
	plant(t, root, rel, os.Getpid(), time.Now().Add(-31*time.Second).Unix())

	bp := lockPath(root, rel) + breakSuffix
	if err := os.WriteFile(bp, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-31 * time.Second)
	if err := os.Chtimes(bp, old, old); err != nil {
		t.Fatal(err)
	}
	release, err := Lock(root, rel)
	if err != nil {
		t.Fatalf("an orphaned break file wedged the path: %v", err)
	}
	release()
}

// TestStalenessHasMarginForCoarseMtime pins the relationship that makes
// mtime-only liveness safe on a filesystem that stores a truncated time.
//
// HFS+, exFAT and several container overlay filesystems keep mtime to one
// or two second granularity, so a lock touched a moment ago can READ as up
// to that much older than it is. The heartbeat refreshes every
// staleAfter/3, so the worst observed age of a live lock is one heartbeat
// interval plus one granularity — and that has to stay comfortably under
// staleAfter or a live lock gets broken on those filesystems.
//
// This is an invariant test, not a filesystem test: it cannot prove
// behaviour on a filesystem this machine does not have. What it does is
// stop somebody shortening staleAfter, or lengthening the heartbeat,
// without noticing that the margin went with it.
//
// proved by: setting staleAfter to 3s — the value brief() uses — fails it,
// which is why brief() is confined to tests that hold a lock briefly.
func TestStalenessHasMarginForCoarseMtime(t *testing.T) {
	const worstGranularity = 2 * time.Second
	worstObserved := staleAfter/3 + worstGranularity
	if worstObserved*2 > staleAfter {
		t.Fatalf("a live lock can read as %v old against a %v window — too little margin for a filesystem with %v mtime granularity",
			worstObserved, staleAfter, worstGranularity)
	}
}

// proved by: comparing against a timestamp stored INSIDE the lock rather
// than the file's mtime makes a truncated mtime irrelevant and this test
// vacuous — which is what the check did before the heartbeat landed.
func TestATruncatedMtimeDoesNotCondemnALiveLock(t *testing.T) {
	root := t.TempDir()
	const rel = ".procoder/state/dispatch.json"
	release, err := Lock(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	// What a coarse filesystem stores: the same instant, rounded down to a
	// two-second boundary.
	p := lockPath(root, rel)
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	coarse := info.ModTime().Truncate(2 * time.Second)
	if err := os.Chtimes(p, coarse, coarse); err != nil {
		t.Fatal(err)
	}

	if stale(p) {
		t.Fatal("a live lock whose mtime was truncated to a 2s boundary was judged stale")
	}
}
