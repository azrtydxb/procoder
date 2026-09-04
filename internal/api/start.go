package api

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// startStale is how long a start lock may go untouched before the process
// that took it is presumed dead. Starting a daemon is a fork and a listen;
// anything older than this belongs to a process that died doing it.
const startStale = 30 * time.Second

// startWait is how long EnsureDaemon waits for the socket to answer before
// giving up. Generous compared to a listen, and bounded: a hook that
// blocked would take the session with it.
const startWait = 2 * time.Second

// EnsureDaemon starts a daemon when nothing is listening, exactly once
// however many sessions ask at the same moment.
//
// Called from the session-start hook, which is the moment a machine has
// something for a daemon to do and the only moment a person is not
// waiting on the answer. There is no launchd, no systemd and no install
// step: a daemon that needed installing would be a setup cost paid by
// every machine to benefit the ones that opted in.
//
// It takes no binary path. The daemon procoder starts is procoder, and
// os.Executable is the only answer to which one: a path from a caller
// would be a parameter that decides what this process executes, which is
// a thing to not have rather than a thing to validate.
//
// Its error is advisory. A session whose daemon would not start is a
// session that runs in-process, which is every session today.
func EnsureDaemon() error {
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("procoder: could not find this binary to start a daemon from (%v)", err)
	}

	path, err := WorkSocket()
	if err != nil {
		return err
	}
	if listening(path) {
		return nil
	}

	lock, err := StartLock()
	if err != nil {
		return err
	}
	dir, err := RunDir()
	if err != nil {
		return err
	}
	release, taken := takeStartLock(lock)
	if !taken {
		// Somebody else is starting one. Waiting for their socket rather
		// than starting a second is the whole point of the lock.
		if waitForSocket(path) {
			return nil
		}
		return fmt.Errorf("procoder: another session is starting the daemon and it has not answered")
	}
	defer release()

	// Checked again under the lock: between the check above and the lock,
	// the process that held it may have finished starting one.
	if listening(path) {
		return nil
	}

	// EnsureDaemon takes no path deliberately: there is no parameter a
	// caller could point at another binary, so the only thing this can
	// start is procoder itself.
	cmd := exec.Command(bin, "serve") // nosemgrep -- bin is os.Executable(), fixed subcommand
	// Never this process's streams: a daemon writing into a hook's stdout
	// would put its own startup line inside the JSON envelope the host
	// parses. But not /dev/null either — a daemon nobody started by hand
	// is exactly the one whose log you need when a command is slow or a
	// machine is not using the daemon it was configured for, and the first
	// version of this discarded it.
	cmd.Stdin = nil
	if log, lerr := os.OpenFile(LogPath(dir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); lerr == nil {
		defer log.Close()
		cmd.Stdout, cmd.Stderr = log, log
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("procoder: could not start the daemon (%v)", err)
	}
	// Not waited on: the daemon outlives this hook by design. Release
	// stops it being this process's zombie.
	go func() { _ = cmd.Wait() }()

	if !waitForSocket(path) {
		return fmt.Errorf("procoder: started the daemon but its socket never answered")
	}
	return nil
}

// listening reports whether something answers at path. A socket file with
// nothing behind it is what a daemon that died leaves, and it is not a
// daemon.
func listening(path string) bool {
	conn, err := net.DialTimeout("unix", path, DialTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func waitForSocket(path string) bool {
	deadline := time.Now().Add(startWait)
	for time.Now().Before(deadline) {
		if listening(path) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// takeStartLock is O_EXCL with a staleness rule, the same shape
// internal/store uses — and for the same reason: go.mod has no require
// block, so a portable flock is not available to spend a dependency on.
func takeStartLock(path string) (release func(), taken bool) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if !staleStartLock(path) {
			return nil, false
		}
		// The holder died mid-start. Remove and try once more; a second
		// failure means somebody else won the race, which is a fine
		// outcome — they are starting the daemon this caller wanted.
		if os.Remove(path) != nil {
			return nil, false
		}
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, false
		}
	}
	fmt.Fprintf(f, "%d\n%d\n", os.Getpid(), time.Now().Unix())
	f.Close()
	return func() { os.Remove(path) }, true
}

func staleStartLock(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		// Unreadable but present: judged by its mtime instead, which is
		// the only thing left to go on.
		if info, serr := os.Stat(path); serr == nil {
			return time.Since(info.ModTime()) > startStale
		}
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		return true // a half-written lock is one nobody is holding
	}
	at, err := strconv.ParseInt(lines[1], 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(at, 0)) > startStale
}
