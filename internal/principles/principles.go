// Package principles owns the engineering principles injected at session
// start: how code should be built in this repository, as opposed to what
// the checks verify. The default is procoder's build philosophy; a repo
// overrides it wholesale with .procoder/PRINCIPLES.md (D-OVERRIDE). The
// binary only prints — the SessionStart hook hands the text to the agent.
package principles

import (
	"os"
	"path/filepath"
	"strings"
)

// File is the repo override path, under D-HOME.
const File = ".procoder/PRINCIPLES.md"

// Default is procoder's build philosophy. Kept short on purpose: it is
// injected into every session.
const Default = `# Engineering principles (procoder)

Build like a senior developer who has been paged at 3am for someone's
cleverness: the best code is the code never written, and the second-best
is boring.

Before writing anything, climb this ladder and stop at the first rung
that holds:

1. Does this need to exist at all? Speculative need = skip it, say so.
2. Does this codebase already have it? Look before you write — a helper
   a few files over beats a fresh reimplementation every time.
3. Does the stdlib do it? Use it.
4. Does the platform do it natively? Prefer the platform.
5. Does an already-installed dependency do it? Never add a new one for
   what a few lines can cover.
6. Can it be one line? One line.
7. Only then: the minimum code that works.

The ladder runs AFTER you understand the problem, never instead of it.
Read the task and every file the change touches, trace the real flow end
to end, then climb. A small diff in the wrong place is not lazy — it is a
second bug. Bug fix = root cause: before editing, find every caller of
what you are about to touch; one guard where all paths converge beats a
patch in the one path the ticket named.

Rules:

- No unrequested abstractions: no interface with one implementation, no
  factory for one product, no config for a value that never changes.
- Deletion over addition; boring over clever.
- Non-trivial logic leaves at least one runnable check behind — the
  smallest thing that fails if the logic breaks.
- Never simplify away: input validation at trust boundaries, error
  handling that prevents data loss, security, accessibility, or anything
  explicitly requested.
- Cut a corner deliberately (a known ceiling: global lock, O(n²) scan,
  naive heuristic)? Mark it in a comment with the configured debt marker
  (default ` + "`debt:`" + `), naming the ceiling AND the condition to
  revisit — ` + "`procoder debt`" + ` harvests these into a ledger, and
  markers with no revisit trigger are flagged as rot.
`

// Effective returns the principles text for this repo: the override file
// if present, else the default; the bool says whether it was overridden.
func Effective(root string) (string, bool) {
	if raw, err := os.ReadFile(filepath.Join(root, File)); err == nil && strings.TrimSpace(string(raw)) != "" {
		return string(raw), true
	}
	return Default, false
}

// Run prints the effective principles and where they came from.
func Run(root string, out func(string)) int {
	text, overridden := Effective(root)
	if overridden {
		out("== engineering principles (repo override: " + File + ")")
	} else {
		out("== engineering principles (procoder default — override with " + File + ")")
	}
	out(strings.TrimRight(text, "\n"))
	return 0
}
