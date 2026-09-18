package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"procoder/internal/api"
	"procoder/internal/config"
	"procoder/internal/copilot"
	"procoder/internal/hook"
	"procoder/internal/initcmd"
)

// viaDaemon answers the command over the socket, or fails saying why.
//
// There is no fallback, deliberately. A machine is either the CLI or the
// server: with [service] mode = "local" the daemon is the only path, and a
// daemon that does not answer is an error the caller is told about — never
// a command that quietly ran somewhere else.
//
// The reason is the same one the rest of this tool is built on. A
// transport that silently ran the work elsewhere would mean two possible
// answers to "where did this verdict come from", indistinguishable from
// the outside, and a machine configured for the server could spend weeks
// never using it with nothing saying so. Failing is louder and shorter.
//
// The second return says whether this took the command. Under mode =
// "local" it is always true — served or failed, the daemon owned it.
func viaDaemon(args []string, s session) (code int, handled bool) {
	if len(args) == 0 || args[0] == "serve" {
		// serve is the daemon. Asking a daemon to start one is a loop.
		return 0, false
	}
	cfg := config.Load(s.root())
	if cfg.ServiceMode != "local" {
		return 0, false // this machine is the CLI
	}

	path, err := api.WorkSocket()
	if err != nil {
		fmt.Fprintln(s.stderr, err.Error())
		return 1, true
	}

	// The four that run what a repository declared are served on the exec
	// socket or not at all. Not a fallback either: this is their route,
	// and where it is closed they are refused rather than run behind the
	// hooks' door.
	if api.Executes(args) {
		if !cfg.ServiceExec {
			fmt.Fprintf(s.stderr,
				"procoder: %s runs what this repository declared, and this machine serves commands from the daemon.\n"+
					"It is served on the exec socket — set `exec = true` under [service] in .procoder/config.toml and\n"+
					"run `procoder serve --exec` — or set `mode = \"off\"` to run commands in this process.\n",
				strings.Join(args, " "))
			return 2, true
		}
		if path, err = api.ExecSocket(); err != nil {
			fmt.Fprintln(s.stderr, err.Error())
			return 1, true
		}
	}

	// The session-start hook is what starts the daemon, so it has to do
	// that BEFORE trying to use one — otherwise a server machine can
	// never get its first daemon: the hook that starts one would be the
	// hook that fails for want of one.
	//
	// Only this hook. It is the moment a machine has something for a
	// daemon to do and the only moment nobody is waiting on a command's
	// answer; every other command finding no daemon is a machine that has
	// not started one, which is a thing to be told rather than papered
	// over.
	if isSessionStart(args) {
		if startErr := api.EnsureDaemon(); startErr != nil {
			fmt.Fprintln(s.stderr, startErr.Error())
		}
	}

	// Read stdin only for the commands that consume it. An open pipe with
	// nothing coming — a CI runner, an agent's shell, a host holding a
	// hook's pipe — never reaches EOF, and io.ReadAll on one waits forever
	// while the daemon looks hung.
	var payload []byte
	if api.ReadsStdin(args) && !copilot.CanAsk(s.stdinFile) && s.stdin != nil {
		b, err := io.ReadAll(s.stdin)
		if err != nil {
			fmt.Fprintf(s.stderr, "procoder: could not read the input for %s (%v)\n", args[0], err)
			return 1, true
		}
		payload = b
	}

	fail := func(err error) (int, bool) {
		fmt.Fprintln(s.stderr, err.Error())
		fmt.Fprintf(s.stderr,
			"This machine runs commands from the daemon ([service] mode = \"local\").\n"+
				"Start it with `procoder serve`, or set `mode = \"off\"` to run commands in this process.\n")
		// The commit gate is the one caller whose failure has to be said
		// in the host's own words. A non-zero exit reads to the host as
		// the hook erroring, and it lets the commit through — so a gate
		// that could not run would wave every commit past itself, which
		// is the silent green this tool exists to prevent. Denying is how
		// you say "this did not run, so it is not passing".
		if isPreToolUse(args) {
			hook.Deny(s.stdout, s.env,
				"procoder gate: this machine runs the gate from the daemon and it could not be reached — "+
					"nothing was checked, so this commit is NOT verified. Start it with `procoder serve`, "+
					"or set [service] mode = \"off\" in .procoder/config.toml.")
			return 0, true
		}
		return 1, true
	}

	res, err := api.Client{Path: path, Version: version}.Do(api.Request{
		Argv:    args,
		Cwd:     s.cwd,
		Env:     s.env,
		Stdin:   payload,
		Confirm: s.confirm,
	})
	if err != nil {
		return fail(err)
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
	// The offsets are what keep a long job's polling linear: the daemon
	// sends only what this caller has not seen.
	var wroteOut, wroteErr int
	for {
		res, err := client.Do(api.Request{Job: id, Since: wroteOut, SinceErr: wroteErr})
		if err != nil {
			// The daemon went away mid-job. The command's own state is
			// unknown, so the caller is told rather than being handed a
			// verdict nobody computed.
			fmt.Fprintf(s.stderr, "procoder: lost the daemon while %s was running (%v)\n", id, err)
			return 1, true
		}
		io.WriteString(s.stdout, res.Stdout)
		wroteOut += len(res.Stdout)
		io.WriteString(s.stderr, res.Stderr)
		wroteErr += len(res.Stderr)
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

// askServerMode puts the local-server question to whoever is there, and
// answers nil when nobody is.
//
// A supplied confirmation counts as somebody: over the API the question
// was already put, wherever the caller put it. A terminal is asked
// directly. Anything else — CI, a pipe, a hook — is nobody, and nobody is
// not a no: it is nothing written at all.
func (s session) askServerMode() *string {
	if s.confirm != nil {
		return s.confirm
	}
	if !copilot.CanAsk(s.stdinFile) {
		return nil
	}
	s.out(initcmd.ServerQuestion)
	line, err := bufio.NewReader(s.stdinFile).ReadString('\n')
	if err != nil && line == "" {
		return nil
	}
	line = strings.TrimSpace(line)
	return &line
}

// isPreToolUse spots the commit gate's hook, whose failure has to be a
// deny rather than an error.
func isPreToolUse(args []string) bool {
	return len(args) > 1 && args[0] == "hook" && args[1] == "pre-tool-use"
}

// isSessionStart spots the hook that starts the daemon.
func isSessionStart(args []string) bool {
	return len(args) > 1 && args[0] == "principles" && args[1] == "--hook"
}
