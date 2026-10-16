//go:build unix

package procgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The shape #310 found: the process procoder starts is a wrapper, and the
// work runs in its child. Cancelling must kill the child too.
// proved by: removed Setpgid and the group kill from prepare — exec's
// default kills the wrapper only and the grandchild sleep outlives the
// cancel, failing the liveness poll below.
func TestCancelKillsTheGrandchild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// sh starts a long sleep as its own child, records its pid, and waits.
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `sleep 60 & echo $! > "$1"; wait`, "sh", pidFile)
	done := make(chan error, 1)
	go func() { done <- Run(cmd) }()

	var pid int
	deadline := time.Now().Add(10 * time.Second)
	for pid == 0 && time.Now().Before(deadline) {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		if pid == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if pid == 0 {
		t.Fatal("the grandchild never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after cancel")
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("grandchild %d survived the cancel", pid)
}

// A tool that finishes on its own is unaffected: its output and exit
// status come back as exec would give them.
// proved by: made Run return nil without waiting — the output is empty.
func TestRunReturnsTheToolsAnswer(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", "echo hi; exit 3")
	var out strings.Builder
	cmd.Stdout = &out
	err := Run(cmd)
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 3 {
		t.Fatalf("want exit 3, got %v", err)
	}
	if out.String() != "hi\n" {
		t.Errorf("output lost: %q", out.String())
	}
}

// A hangup procoder was started to ignore (nohup) must not kill the tool.
// proved by: notified SIGHUP unconditionally — Notify un-ignores it, the
// watcher kills the group on the hangup the tool sends, and Run fails.
func TestAnIgnoredSignalStaysIgnored(t *testing.T) {
	signal.Ignore(syscall.SIGHUP)
	defer signal.Reset(syscall.SIGHUP)
	cmd := exec.CommandContext(context.Background(), "/bin/sh", "-c", `kill -HUP "$1"; sleep 0.3; exit 0`, "sh", strconv.Itoa(os.Getpid()))
	if err := Run(cmd); err != nil {
		t.Fatalf("the tool was killed by a hangup procoder ignores: %v", err)
	}
	if !signal.Ignored(syscall.SIGHUP) {
		t.Fatal("SIGHUP is no longer ignored after the run")
	}
}
