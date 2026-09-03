package main

import (
	"errors"
	"fmt"
	"io"
	"time"

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
	if res.Job != nil && res.Exit == nil {
		return followJob(res.Job.ID, path, s)
	}
	io.WriteString(s.stdout, res.Stdout)
	io.WriteString(s.stderr, res.Stderr)
	if res.Exit == nil {
		return 0, true
	}
	return *res.Exit, true
}

// followJob waits out a long-running command and writes what it produces
// as it appears.
//
// A CLI caller asked for `procoder test` and expects `procoder test`: the
// job exists so the SOCKET does not have to hold a suite open, not so the
// person has to poll. What a client with something else to do gets is the
// job id; what the CLI gets is the command it asked for.
//
// Only the new bytes are written on each poll, because a poll returns
// everything accumulated so far and reprinting it would repeat the suite's
// output once per tick.
func followJob(id, path string, s session) (int, bool) {
	client := api.Client{Path: path, Version: version}
	var wroteOut, wroteErr int
	for {
		res, err := client.Do(api.Request{Job: id})
		if err != nil {
			// The daemon went away mid-job. The command's own state is
			// unknown, so the caller is told rather than being handed a
			// verdict nobody computed — and it is NOT re-run in-process,
			// which could run a release or a suite twice.
			fmt.Fprintf(s.stderr, "procoder: lost the daemon while %s was running (%v)\n", id, err)
			return 1, true
		}
		if len(res.Stdout) > wroteOut {
			io.WriteString(s.stdout, res.Stdout[wroteOut:])
			wroteOut = len(res.Stdout)
		}
		if len(res.Stderr) > wroteErr {
			io.WriteString(s.stderr, res.Stderr[wroteErr:])
			wroteErr = len(res.Stderr)
		}
		if res.Job != nil && res.Job.State == api.JobLost {
			fmt.Fprintf(s.stderr, "procoder: job %s is gone — the daemon restarted while it was running\n", id)
			return 1, true
		}
		if res.Exit != nil {
			return *res.Exit, true
		}
		time.Sleep(jobPollInterval)
	}
}

// jobPollInterval is short enough that a suite's output does not feel
// batched, and long enough that following one is not a busy loop.
const jobPollInterval = 100 * time.Millisecond

// ensureDaemon starts the local daemon where a machine asked for one and
// none is listening.
//
// Called from the session-start hook: the moment a machine has something
// for a daemon to do, and the only moment nobody is waiting on a command's
// answer. Every failure is silent to stdout — a session whose daemon would
// not start is a session that runs in-process, which is every session
// today, and a hook that printed about it would put its noise inside the
// JSON envelope three of the four hosts parse.
func (s session) ensureDaemon() {
	if config.Load(s.root()).ServiceMode != "local" {
		return
	}
	if err := api.EnsureDaemon(); err != nil {
		fmt.Fprintln(s.stderr, err.Error())
	}
}
