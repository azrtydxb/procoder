//go:build unix && !linux

package procgroup

import "syscall"

// debt: on macOS and the BSDs a SIGKILLed procoder still leaves its tool's
// group running — there is no parent-death signal to ask for. Ceiling: only
// an uncatchable kill of procoder itself; cancel and SIGINT/SIGTERM/SIGHUP
// all take the group down. Revisit if the hook host is seen SIGKILLing
// procoder, or when the scans move into the daemon, which outlives them.
func setDeathSignal(*syscall.SysProcAttr) {}
