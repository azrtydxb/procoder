package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// tempPrefix marks the half-written files. It is fixed rather than random so
// a sweep can recognise one left behind by a process that died mid-write —
// and so a person reading `git status` after a crash can tell what it is.
const tempPrefix = ".procoder-tmp-"

// tempMaxAge is how long a temp file may sit before the sweep removes it. It
// is not the lock's staleAfter by accident: both answer the same question,
// which is how long a live write could plausibly still be going.
const tempMaxAge = 30 * time.Second

// forceRenameFailure makes the rename fail. It exists for one test and
// nothing else: there is no portable way to make os.Rename fail on demand,
// and the atomicity claim is not worth making if nothing can check it.
var forceRenameFailure bool

// ReadFile reads the file at the repo-relative path. An absent file returns
// an error satisfying os.IsNotExist, exactly as os.ReadFile does, because
// every caller already treats that as "none" rather than as a fault.
func ReadFile(root, relPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
}

// WriteFile replaces the file at the repo-relative path with data, or leaves
// it exactly as it was.
//
// Every write in procoder was an os.WriteFile until now, which truncates
// first: a process killed mid-write leaves a truncated claims.json, and the
// next reader reports a corrupt ledger for a crash that had nothing to do
// with it. Here the bytes land in a temp file, are flushed to disk, and are
// renamed over the target — so a reader sees the whole old file or the whole
// new one, and a failure leaves the old one untouched.
//
// The temp file goes in the DESTINATION directory, not os.TempDir: rename is
// only atomic within a filesystem, and /tmp is frequently a different one.
func WriteFile(root, relPath string, data []byte, perm os.FileMode) error {
	target := filepath.Join(root, filepath.FromSlash(relPath))
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("procoder: could not create %s for %s (%v) — the write was NOT made", dir, relPath, err)
	}

	f, err := os.CreateTemp(dir, tempPrefix+"*")
	if err != nil {
		return fmt.Errorf("procoder: could not stage a write to %s (%v) — the write was NOT made", relPath, err)
	}
	tmp := f.Name()
	// From here every failure removes the temp file. The target is not open
	// and cannot have been damaged, which is the property being bought.
	fail := func(err error) error {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}

	if _, err := f.Write(data); err != nil {
		return fail(fmt.Errorf("procoder: could not write %s (%v) — the write was NOT made", relPath, err))
	}
	// Sync before the rename: a rename that outlives its own data is how a
	// crash turns an atomic write into an empty file.
	if err := f.Sync(); err != nil {
		return fail(fmt.Errorf("procoder: could not flush %s (%v) — the write was NOT made", relPath, err))
	}
	if err := f.Chmod(perm); err != nil {
		return fail(fmt.Errorf("procoder: could not set the mode on %s (%v) — the write was NOT made", relPath, err))
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("procoder: could not close %s (%v) — the write was NOT made", relPath, err)
	}

	if err := rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("procoder: could not replace %s (%v) — the write was NOT made", relPath, err)
	}

	sweep(dir)
	return nil
}

// rename is os.Rename with the test seam in front of it.
func rename(from, to string) error {
	if forceRenameFailure {
		return errors.New("rename failure forced by a test")
	}
	return os.Rename(from, to)
}

// sweep removes temp files old enough that no live write could still own
// them — the litter left by a process that died between staging and rename.
//
// Best effort by design: it runs after a write that has already succeeded,
// and failing the caller because some unrelated stale file could not be
// removed would turn tidying into an outage.
func sweep(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-tempMaxAge)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}
