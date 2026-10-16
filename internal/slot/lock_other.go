//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package slot

import "os"

// debt: no flock here, so every slot is always free and the machine-wide
// bound does not apply off the flock platforms. Ceiling: concurrent scans
// on Windows are as unbounded as they were before #310. Revisit when a
// Windows user runs SAST at scale, with LockFileEx via golang.org/x/sys.
func tryLock(*os.File) (bool, error) { return true, nil }

func unlock(*os.File) error { return nil }

// Supported reports whether slots actually bound anything on this platform.
const Supported = false
