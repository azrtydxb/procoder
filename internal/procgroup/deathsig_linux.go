//go:build linux

package procgroup

import "syscall"

// setDeathSignal kills the tool's leader if procoder itself dies without a
// chance to clean up — SIGKILL, which no handler sees. Linux only; the
// leader's own children are reached by killGroup on every other path.
func setDeathSignal(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
