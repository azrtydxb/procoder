//go:build unix

package procgroup

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	setDeathSignal(cmd.SysProcAttr)
	cmd.Cancel = func() error { return killGroup(cmd) }
}

// killGroup kills every process in the group cmd leads. The group's id is
// the leader's pid, because Setpgid made it so.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// watchSignals kills the tool's group when procoder is told to stop, and
// hands the signal back once the tool is gone, so procoder still stops.
// The returned func ends the watch; it must be called after Wait.
func watchSignals(cmd *exec.Cmd) func() {
	ch := make(chan os.Signal, 1)
	// A signal procoder inherited as ignored (nohup, a background job) stays
	// ignored: Notify would otherwise un-ignore it, and a hangup the user
	// asked to survive would kill the scan.
	var sigs []os.Signal
	for _, s := range []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		if !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	if len(sigs) == 0 {
		return func() {}
	}
	signal.Notify(ch, sigs...)
	done := make(chan struct{})
	finished := make(chan struct{})
	var got os.Signal
	go func() {
		defer close(finished)
		select {
		case got = <-ch:
			_ = killGroup(cmd)
		case <-done:
		}
	}()
	return func() {
		close(done)
		<-finished
		signal.Stop(ch)
		if got == nil {
			// A signal that raced the tool's exit, or arrived after the
			// watcher returned, is still in ch: it is honoured, not lost.
			select {
			case got = <-ch:
			default:
			}
		}
		if s, ok := got.(syscall.Signal); ok {
			// Re-raised after Stop: with no watcher left the default
			// action applies, and procoder ends the way it was asked to.
			// A watcher still registered elsewhere receives it instead
			// and does the same once its own tool is down.
			_ = syscall.Kill(os.Getpid(), s)
		}
	}
}
