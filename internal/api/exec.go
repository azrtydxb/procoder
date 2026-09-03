package api

// Executes reports whether an argv runs a command the repository — or a
// prior agent session — declared, rather than merely reading one.
//
// Four do. They are the whole of the "look but don't run" boundary
// (#201): procoder reads plenty that an agent session could have written
// — the ask ledger, the handoff note, the backlog, the specs — and hooks
// run unattended on every write and every commit. A path that executes
// any of it must be reachable only by a separate step a person invoked.
//
// The socket does not change that and cannot be trusted to. Its 0600 mode
// authenticates the USER, not the process: every process running as that
// user can open it, including an agent session's own shell. So these four
// are served on the exec socket or not at all, and the client never offers
// them to the work socket.
func Executes(argv []string) bool {
	if len(argv) == 0 {
		return false
	}
	switch argv[0] {
	case "self-upgrade":
		// Installs a binary over this one. There is no read-only form.
		return true
	case "evidence":
		// `evidence record <cmd>` runs an arbitrary command. `evidence`
		// with anything else does not.
		return len(argv) > 1 && argv[1] == "record"
	case "run":
		// `run` prints the declared launch commands; only --exec runs one.
		return hasFlag(argv, "--exec")
	case "init":
		// `init` prints the install commands; only --yes runs them.
		return hasFlag(argv, "--yes")
	}
	return false
}

func hasFlag(argv []string, flag string) bool {
	for _, a := range argv[1:] {
		if a == flag {
			return true
		}
	}
	return false
}
