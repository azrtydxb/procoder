package portability

import (
	"fmt"
	"os"
	"path/filepath"

	"procoder/internal/gitx"
)

// verdict is what one host's rule file is: ok, missing, drifted, or
// unreadable. Both `procoder agents` and the gate ask for it, so the
// command and the gate cannot disagree about what drift is.
type verdict int

const (
	ok verdict = iota
	missing
	drifted
	unreadable
)

// check reads one host's copy and compares it with the master body.
func check(root string, c Copy, want string) (verdict, error) {
	raw, err := os.ReadFile(filepath.Join(root, c.Path))
	switch {
	case err != nil && !os.IsNotExist(err):
		return unreadable, err
	case err != nil:
		return missing, nil
	case normalize(stripFrontmatter(string(raw))) != want:
		return drifted, nil
	}
	return ok, nil
}

// AgentsDrift reports rule files that no longer match AGENTS.md.
//
// `procoder agents` has always ended by printing "the gate blocks on
// drift", and docs/commands.md says the same. Neither was true: nothing
// in the gate, the hooks or CI ever asked. So every other host — Cursor,
// Windsurf, Cline, Kilo, Roo, Kiro, Codex and the rest — could be reading
// rules that had drifted from AGENTS.md while procoder reported clean,
// which is the failure mode the agent layer exists to prevent.
//
// Blocking, because a stale rule file is not a matter of taste: it is
// another agent being told something this repository stopped believing.
// A repository with no AGENTS.md ships no agent layer and gets nothing.
func AgentsDrift(root string) []gitx.Finding {
	master, err := os.ReadFile(filepath.Join(root, Master))
	if err != nil {
		return nil
	}
	want := normalize(stripFrontmatter(string(master)))
	var out []gitx.Finding
	for _, c := range Copies {
		v, rerr := check(root, c, want)
		switch v {
		case unreadable:
			out = append(out, gitx.Finding{Blocking: true, File: c.Path,
				Message: fmt.Sprintf("%s rule file is unreadable (%v) — NOT checked against %s (agents)", c.Host, rerr, Master)})
		case missing:
			out = append(out, gitx.Finding{Blocking: true, File: c.Path,
				Message: fmt.Sprintf("%s has no rule file — run `procoder agents` for the content to write (agents)", c.Host)})
		case drifted:
			out = append(out, gitx.Finding{Blocking: true, File: c.Path,
				Message: fmt.Sprintf("%s rule file has drifted from %s — that host is reading rules this repository no longer holds; run `procoder agents` (agents)", c.Host, Master)})
		}
	}
	return out
}
