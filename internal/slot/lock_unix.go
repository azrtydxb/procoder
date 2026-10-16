//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package slot

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes an exclusive flock without waiting. Two opens of the same
// file are separate locks even inside one process, which is what lets the
// tests hold a slot against the code under test.
func tryLock(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return false, err
}

func unlock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

// Supported reports whether slots actually bound anything on this platform.
const Supported = true
