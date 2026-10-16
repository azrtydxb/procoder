// Package procgroup runs an external tool so that stopping it stops all of
// it.
//
// exec.CommandContext kills the process it started and nothing else. A
// scanner is rarely one process: `semgrep` is a Python wrapper whose real
// work happens in `semgrep-core`, its child. Cancelling the context killed
// the wrapper and left the child running to completion — minutes of CPU on
// a large tree, for a verdict nobody was waiting for any more (#310).
//
// Run puts the tool in a process group of its own and, on cancel, kills the
// group. The tool's own group also takes it out of procoder's, so a signal
// aimed at procoder's group — `timeout 600 procoder check` sends one —
// would no longer reach it; Run therefore watches for the terminating
// signals while the tool runs, kills the group, and then lets the signal
// do to procoder what it would have done anyway.
package procgroup

import (
	"os/exec"
	"time"
)

// waitDelay bounds how long Wait keeps reading the tool's output after the
// group was killed. A grandchild that inherited the pipe and somehow
// survived must not hold the caller open indefinitely.
const waitDelay = 5 * time.Second

// Run starts cmd in a process group of its own and waits for it. cmd must
// come from exec.CommandContext: cancelling that context kills the whole
// group, not just its leader.
func Run(cmd *exec.Cmd) error {
	prepare(cmd)
	cmd.WaitDelay = waitDelay
	if err := cmd.Start(); err != nil {
		return err
	}
	stop := watchSignals(cmd)
	err := cmd.Wait()
	stop()
	return err
}
