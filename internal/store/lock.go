// Package store is the one place procoder reads and writes .procoder/.
//
// Until now every domain issued its own os.ReadFile and os.WriteFile
// against its own path constant — twenty-six of them, none locking, none
// atomic. That was correct while procoder was a short-lived process: one
// hook ran, touched a file, and exited before the next one started. The
// daemon in #117 ends it. Two sessions writing the same ledger is ordinary
// there, and a read-modify-write with no lock loses one of the two updates
// silently.
//
// This file is the lock. It is a lockfile rather than flock because
// go.mod has no require block and procoder is not spending its first
// dependency on golang.org/x/sys — which is what a portable
// flock/LockFileEx pair would cost.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// lockDir sits under .procoder/state/, which is already gitignored. A
	// lock file beside the file it protects would show up in git status
	// and in review, which is how a lock becomes a diff.
	lockDir   = ".procoder/state/locks"
	lockRetry = 10 * time.Millisecond
	// breakSuffix names the file that serialises BREAKING a stale lock.
	// Judging a lock stale and removing it are two operations, and two
	// callers doing them at once is how both end up holding: the first
	// removes and re-creates, the second removes what the first just
	// created. Only the holder of this file may break, so that interleave
	// cannot happen.
	breakSuffix = ".break"
)

// staleAfter is how long a lock may go untouched before it is presumed
// dead. A HELD lock is touched by its heartbeat every staleAfter/3, so
// reaching this means the owner stopped running — not that it is slow.
//
// A var rather than a const only so a test can shorten it; nothing in
// procoder changes it at runtime.
var staleAfter = 30 * time.Second

// heartbeatEvery keeps a held lock looking alive. Without it a write that
// legitimately takes longer than staleAfter — rewriting a multi-megabyte
// tags.jsonl on a slow filesystem — would have its lock taken out from
// under it while it was still writing.
func heartbeatEvery() time.Duration { return staleAfter / 3 }

// lockTimeout bounds the wait. A hook that blocked would take the session
// with it, so waiting forever is never an option here.
//
// A var rather than a const only so the contention tests can shorten it;
// six tests waiting out five seconds each is thirty seconds of sleeping in
// a suite that otherwise runs in two.
var lockTimeout = 5 * time.Second

// Notice is where the store says it had to break a stale lock. Breaking one
// means a previous run died holding it, which is worth a human knowing;
// returning the fact to callers that all discard it would be an honesty
// channel nobody reads.
var Notice io.Writer = os.Stderr

// held is one acquired lock: which path, and WHICH FILE — os.SameFile
// against this is what stops release removing a lock that has since been
// broken and replaced by somebody else's live one.
type held struct {
	rel  string
	info os.FileInfo
	stop chan struct{}
}

// lockPath is where rel's lock lives. The name is a hash because the
// repo-relative path contains separators, and because a name derived from
// user content has to be safe on three filesystems.
func lockPath(root, rel string) string {
	sum := sha256.Sum256([]byte(rel))
	return filepath.Join(root, filepath.FromSlash(lockDir), hex.EncodeToString(sum[:])[:16]+".lock")
}

// Lock takes an exclusive lock on each repo-relative path and returns the
// func that releases them.
//
// The paths are locked in sorted order, always, whatever order the caller
// asked for. That is the whole deadlock story: two callers wanting the
// same two files can no longer take them in opposite orders.
//
// On failure nothing is held: locks already taken are released before the
// error returns, so a caller that gives up does not leave a file locked
// against everybody else for the next thirty seconds.
func Lock(root string, relPaths ...string) (release func(), err error) {
	paths := append([]string(nil), relPaths...)
	sort.Strings(paths)

	var hs []held
	rel := func() {
		for _, h := range hs {
			close(h.stop)
			p := lockPath(root, h.rel)
			// Remove only what we actually hold. If this lock was broken
			// as stale and somebody else's live lock now sits at the same
			// path, removing it would hand a third caller a lock that is
			// already held.
			if cur, err := os.Stat(p); err == nil && os.SameFile(cur, h.info) {
				_ = os.Remove(p)
			}
		}
	}
	for _, p := range paths {
		h, err := acquire(root, p)
		if err != nil {
			rel()
			return nil, err
		}
		hs = append(hs, h)
	}
	return rel, nil
}

// acquire takes one lock, breaking a dead one if that is what is in the
// way, and gives up at the deadline rather than waiting on a live writer
// forever.
func acquire(root, rel string) (held, error) {
	p := lockPath(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return held{}, fmt.Errorf("procoder: could not create the lock directory for %s (%v) — the write was NOT made", rel, err)
	}

	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().Unix())
			cerr := f.Close()
			info, serr := os.Stat(p)
			if werr != nil || cerr != nil || serr != nil {
				// A lock nobody can read is one everybody else will break.
				_ = os.Remove(p)
				return held{}, fmt.Errorf("procoder: could not write the lock for %s — the write was NOT made", rel)
			}
			h := held{rel: rel, info: info, stop: make(chan struct{})}
			go keepAlive(p, h.stop)
			return h, nil
		}
		if !os.IsExist(err) {
			return held{}, fmt.Errorf("procoder: could not lock %s (%v) — the write was NOT made", rel, err)
		}

		if !breakStale(p, rel) {
			time.Sleep(lockRetry)
		}
		if time.Now().After(deadline) {
			return held{}, fmt.Errorf("procoder: could not lock %s within %s — the write was NOT made", rel, lockTimeout)
		}
	}
}

// breakStale removes the lock at p if it is dead, and reports whether it
// did. Only one caller at a time may be inside the removal, because
// judging and removing are two operations and interleaving two of them is
// how both end up holding.
func breakStale(p, rel string) bool {
	bp := p + breakSuffix
	b, err := os.OpenFile(bp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Somebody else is breaking this one — unless they died doing it,
		// which takes microseconds, so an old break file is litter. The
		// recovery is bounded: the worst case is two callers breaking a
		// lock that is dead either way, and O_EXCL still gives one winner.
		if info, serr := os.Stat(bp); serr == nil && time.Since(info.ModTime()) > staleAfter {
			_ = os.Remove(bp)
		}
		return false
	}
	_ = b.Close()
	defer func() { _ = os.Remove(bp) }()

	if !stale(p) {
		return false
	}
	if os.Remove(p) != nil {
		return false
	}
	fmt.Fprintf(Notice, "procoder: broke a stale lock on %s — a previous run left it behind\n", rel)
	return true
}

// keepAlive touches the lock while it is held, so a slow write is never
// mistaken for a dead one.
func keepAlive(p string, stop <-chan struct{}) {
	t := time.NewTicker(heartbeatEvery())
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			_ = os.Chtimes(p, now, now)
		}
	}
}

// stale says whether the lock at p can be presumed dead.
//
// The file's OWN age is the test that matters, and the heartbeat above is
// what makes it trustworthy: a lock whose owner is running is touched
// every ten seconds, so one untouched for thirty belongs to a process that
// is gone.
//
// The contents get a say only when they parse. A lock file is created
// empty by O_EXCL and written a moment later, so for that moment every
// live lock is unparsable; treating that as dead let a second caller steal
// a newborn lock and gave two holders — found by
// TestConcurrentAppendsBothSurvive. A genuinely corrupt lock is condemned
// by its mtime instead, thirty seconds later.
//
// A lock that has vanished is NOT stale — it is gone, and the next O_EXCL
// settles who gets it.
func stale(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	if time.Since(info.ModTime()) > staleAfter {
		return true
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		return false // being written right now
	}
	if _, err := strconv.Atoi(fields[0]); err != nil {
		return false
	}
	sec, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return false
	}
	// The recorded time is when the lock was TAKEN, and nothing refreshes
	// it — the heartbeat touches the file's mtime, which is the liveness
	// signal. Ageing the contents as well would condemn every lock whose
	// owner is still working after staleAfter, which is the case the
	// heartbeat exists to protect.
	//
	// What the contents still answer is a timestamp in the FUTURE: a clock
	// that moved, or a restored backup. A lock whose owner claims to have
	// taken it after now cannot be believed about anything.
	return time.Since(time.Unix(sec, 0)) < 0
}
