package main

import (
	"bytes"
	"io"
	"os"

	"procoder/internal/api"
	"procoder/internal/host"
)

// apiRunner is the command layer's answer to a request: build the session
// the request describes, and run exactly what the CLI runs.
//
// Every difference between the two doors lives in this function, and there
// are only three of them — the streams are buffers, the directory and
// environment come from the caller rather than the process, and there is
// no file handle at either end. The third is not a limitation to work
// around: over a socket there is genuinely no terminal and no redirect,
// and nil is what says so.
func apiRunner(req api.Request, stdout, stderr io.Writer) (int, *api.Result) {
	env := host.Env{}
	for k, v := range req.Env {
		env[k] = v
	}
	cwd := req.Cwd
	if cwd == "" {
		// A request with no directory is answered where the daemon
		// stands, which is almost certainly not what the caller meant —
		// but refusing here would make the envelope's simplest possible
		// request an error, and every command already handles a directory
		// that is not a repository.
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	col := &collector{}
	code := run(req.Argv, session{
		stdin:   bytes.NewReader(req.Stdin),
		stdout:  stdout,
		stderr:  stderr,
		cwd:     cwd,
		env:     env,
		col:     col,
		confirm: req.Confirm,
	})
	return code, col.result()
}
