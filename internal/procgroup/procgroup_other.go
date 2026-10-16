//go:build !unix

package procgroup

import "os/exec"

// debt: off unix the tool is not given a group of its own, so cancel kills
// only the process procoder started (exec's default). Ceiling: a tool whose
// work runs in a grandchild keeps running after a cancel on Windows.
// Revisit when a Windows user reports it, with a Job Object
// (golang.org/x/sys/windows) as the fix.
func prepare(*exec.Cmd) {}

func watchSignals(*exec.Cmd) func() { return func() {} }
