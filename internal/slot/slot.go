// Package slot bounds how many copies of an expensive job run on one
// machine at a time, across every procoder process on it.
//
// A slot is an advisory lock on a file in a shared directory: `name-0` to
// `name-(n-1)`. Taking one waits until a slot is free or the caller's
// context ends. The kernel drops a lock when its holder exits however it
// exits, so a crashed procoder never leaves a slot taken.
package slot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// poll is how often a waiting caller retries. Scans take seconds to
// minutes; a quarter second of latency on the handover is noise.
const poll = 250 * time.Millisecond

// ErrBusy is returned when ctx ended before a slot came free.
var ErrBusy = errors.New("every slot stayed taken")

// Dir is the machine-wide directory slots live in: the user's cache
// directory, or the temp directory when there is none.
func Dir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "procoder", "slots")
	}
	return filepath.Join(os.TempDir(), "procoder-slots")
}

// Acquire takes one of n slots called name in dir, waiting until one is
// free or ctx is done. It returns ErrBusy (wrapping ctx's error) when the
// wait ran out, and another error when the lock files could not be used at
// all — the caller decides whether that is worth refusing work over.
func Acquire(ctx context.Context, dir, name string, n int) (release func(), err error) {
	if n < 1 {
		n = 1
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	files := make([]*os.File, n)
	defer func() {
		for _, f := range files {
			if f != nil {
				_ = f.Close()
			}
		}
	}()
	for i := range files {
		f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%s-%d", name, i)), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		files[i] = f
	}
	for {
		for i, f := range files {
			ok, err := tryLock(f)
			if err != nil {
				return nil, err
			}
			if ok {
				files[i] = nil // kept open: closing it is the release
				return func() { _ = unlock(f); _ = f.Close() }, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrBusy, ctx.Err())
		case <-time.After(poll):
		}
	}
}
