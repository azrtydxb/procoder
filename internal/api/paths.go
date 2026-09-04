package api

import (
	"fmt"
	"os"
	"path/filepath"
)

// The sockets live under the user's home rather than inside any
// repository. One daemon serves many checkouts, and a socket in one of
// them would make that repository's presence a dependency of every other
// repository's hooks.
const (
	runDirName     = ".procoder"
	runSubdir      = "run"
	workSocketName = "procoder.sock"
	execSocketName = "procoder-exec.sock"
	startLockName  = "start.lock"
	logName        = "serve.log"
)

// LogPath is where a daemon nobody started by hand writes what it served.
func LogPath(runDir string) string { return filepath.Join(runDir, logName) }

// RunDir is where the sockets and the start lock live, created 0700.
//
// The mode is the authentication. There is no port and no token: a unix
// socket is a filesystem object, so the permission bits already answer
// "who may talk to this daemon", and every other scheme would be a second
// answer to a question the filesystem had settled.
func RunDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("procoder: no home directory to put the socket in (%v)", err)
	}
	dir := filepath.Join(home, runDirName, runSubdir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("procoder: could not create %s (%v)", dir, err)
	}
	// MkdirAll leaves an existing directory's mode alone, and one created
	// by an older build — or by a umask — may be wider than 0700. The
	// socket's own 0600 is the real guard, but a readable directory leaks
	// which repositories are being served.
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("procoder: could not secure %s (%v)", dir, err)
	}
	return dir, nil
}

// WorkSocket serves every command except the four that execute.
func WorkSocket() (string, error) { return socketPath(workSocketName) }

// ExecSocket serves only those four, and only where the repository asked
// for it. A separate door rather than a flag on this one: the hook
// transport is never told this path, so a path that runs what a repository
// declared cannot be reached by something running unattended.
func ExecSocket() (string, error) { return socketPath(execSocketName) }

// StartLock serialises auto-start, so ten sessions starting at once leave
// one daemon.
func StartLock() (string, error) { return socketPath(startLockName) }

func socketPath(name string) (string, error) {
	dir, err := RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
