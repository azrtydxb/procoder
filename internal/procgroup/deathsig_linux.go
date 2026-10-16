//go:build linux

package procgroup

import "syscall"

// setDeathSignal kills the tool's leader if procoder itself dies without a
// chance to clean up — SIGKILL, which no handler sees. Linux only, and the
// leader only: its own children (semgrep-core) outlive a SIGKILLed procoder
// as orphans. killGroup reaches them on every other path.
func setDeathSignal(attr *syscall.SysProcAttr) {
	attr.Pdeathsig = syscall.SIGKILL
}
