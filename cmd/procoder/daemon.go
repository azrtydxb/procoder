package main

import (
	"errors"
	"fmt"
	"io"

	"procoder/internal/api"
	"procoder/internal/config"
	"procoder/internal/copilot"
)

// tryDaemon answers the command over the socket where a machine asked for
// a daemon, and says it could not otherwise.
//
// The fallback is the whole contract. Every failure here — no daemon, a
// daemon from another build, a socket that went away mid-request — costs
// the caller the daemon's speed and never its answer: the command runs
// in-process, exactly as it does on a machine that never had one. A
// transport whose failure could cost a verdict would be a worse gate, not
// a faster one.
//
// The four executing commands are never sent. They are served on the exec
// socket or not at all, and a client that offered them here would put a
// path that runs what a repository declared behind the same door the hooks
// use.
func tryDaemon(args []string, s session) (int, bool) {
	if len(args) == 0 || args[0] == "serve" {
		// serve is the daemon. Asking a daemon to start one is a loop.
		return 0, false
	}
	cfg := config.Load(s.root())
	if cfg.ServiceMode != "local" {
		return 0, false
	}
	if api.Executes(args) {
		return 0, false
	}

	path, err := api.WorkSocket()
	if err != nil {
		return 0, false
	}

	// Read stdin only where there is something to read. A terminal has
	// nothing waiting and ReadAll on one blocks forever — which would turn
	// every interactive command on a daemon-configured machine into a
	// hang, and the daemon is supposed to be invisible.
	var stdin []byte
	if copilot.CanAsk(s.stdinFile) {
		// A character device that is not /dev/null: a person is there and
		// whatever they type belongs to the command, not to this read.
		stdin = nil
	} else if s.stdin != nil {
		b, err := io.ReadAll(s.stdin)
		if err != nil {
			return 0, false
		}
		stdin = b
	}

	res, err := api.Client{Path: path, Version: version}.Do(api.Request{
		Argv:    args,
		Cwd:     s.cwd,
		Env:     s.env,
		Stdin:   stdin,
		Confirm: s.confirm,
	})
	if err != nil {
		// Said out loud, at the level the failure deserves: a machine that
		// configured a daemon and is not getting one should be able to see
		// that, and a hook must not be told about it on every write.
		if !errors.Is(err, api.ErrNoDaemon) {
			fmt.Fprintln(s.stderr, err.Error()+" — running in-process")
		}
		return 0, false
	}
	io.WriteString(s.stdout, res.Stdout)
	io.WriteString(s.stderr, res.Stderr)
	if res.Exit == nil {
		return 0, true
	}
	return *res.Exit, true
}
