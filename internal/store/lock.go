// Package store is the one place procoder reads and writes .procoder/.
//
// Until now every domain issued its own os.ReadFile and os.WriteFile
// against its own path constant — twenty-five of them, none locking, none
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
	lockDir = ".procoder/state/locks"
	// staleAfter is how long a lock may sit before it is presumed dead. No
	// write through this package holds a lock for anything like this long;
	// what takes this long is a process that was killed holding one.
	staleAfter = 30 * time.Second
	// lockTimeout bounds the wait. A hook that blocked would take the
	// session with it, so waiting forever is never an option here.
	lockTimeout = 5 * time.Second
	lockRetry   = 10 * time.Millisecond
)

// lockPath is where rel's lock lives. The name is a hash because the
// repo-relative path contains separators, and because a name derived from
// user content has to be safe on three filesystems.
func lockPath(root, rel string) string {
	sum := sha256.Sum256([]byte(rel))
	return filepath.Join(root, filepath.FromSlash(lockDir), hex.EncodeToString(sum[:])[:16]+".lock")
}

// Lock takes an exclusive lock on each repo-relative path and returns the
// func that releases them, plus any stale lock it had to break so the
// caller can report it.
//
// The paths are locked in sorted order, always, whatever order the caller
// asked for. That is the whole deadlock story: two callers wanting the
// same two files can no longer take them in opposite orders.
//
// broken is returned rather than read from a package-level accessor,
// because concurrent callers would race on shared state — and this package
// exists precisely because that concurrency is real.
//
// On failure nothing is held: locks already taken are released before the
// error returns, so a caller that gives up does not leave a file locked
// against everybody else for the next thirty seconds.
func Lock(root string, relPaths ...string) (release func(), broken []string, err error) {
	paths := append([]string(nil), relPaths...)
	sort.Strings(paths)

	var held []string
	rel := func() {
		for _, p := range held {
			_ = os.Remove(lockPath(root, p))
		}
	}
	for _, p := range paths {
		b, err := acquire(root, p)
		broken = append(broken, b...)
		if err != nil {
			rel()
			return nil, nil, err
		}
		held = append(held, p)
	}
	return rel, broken, nil
}

// acquire takes one lock, breaking a dead one if that is what is in the
// way, and gives up at the deadline rather than waiting on a live writer
// forever.
func acquire(root, rel string) ([]string, error) {
	p := lockPath(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, fmt.Errorf("procoder: could not create the lock directory for %s (%v) — the write was NOT made", rel, err)
	}

	var broken []string
	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().Unix())
			cerr := f.Close()
			if werr != nil || cerr != nil {
				// A lock nobody can read is one everybody else will break.
				_ = os.Remove(p)
				return broken, fmt.Errorf("procoder: could not write the lock for %s — the write was NOT made", rel)
			}
			return broken, nil
		}
		if !os.IsExist(err) {
			return broken, fmt.Errorf("procoder: could not lock %s (%v) — the write was NOT made", rel, err)
		}

		if stale(p) {
			_ = os.Remove(p)
			if len(broken) == 0 {
				broken = append(broken, rel)
			}
		} else {
			time.Sleep(lockRetry)
		}
		if time.Now().After(deadline) {
			return broken, fmt.Errorf("procoder: could not lock %s within %s — the write was NOT made", rel, lockTimeout)
		}
	}
}

// stale says whether the lock at p can be presumed dead.
//
// Three ways, and two of them are about not being able to tell: contents
// that do not parse, and a timestamp in the future, are both locks whose
// liveness cannot be established. A lock nobody can interpret cannot be
// proven live, and treating it as live would wedge the file until somebody
// deleted it by hand.
//
// A lock that has vanished is NOT stale — it is gone, and the next O_EXCL
// settles who gets it.
func stale(p string) bool {
	raw, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 {
		return true
	}
	if _, err := strconv.Atoi(fields[0]); err != nil {
		return true
	}
	sec, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return true
	}
	age := time.Since(time.Unix(sec, 0))
	return age > staleAfter || age < 0
}
