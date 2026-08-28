package analysis

import (
	"fmt"
	"path/filepath"

	"procoder/internal/store"
)

// Entry is a point in the quality chain a change can start from.
type Entry struct {
	Name string
	When string
	Next string
}

// Entries are the chain's entry points, widest first.
//
// D-5: right-sizing is naming which one a change belongs at, not removing
// enforcement. The premise procoder is often described by — that every
// change must run spec → plan → backlog → sprint — does not survive
// contact with the code: no gate finding requires a spec, and this
// repository routinely lands fixes that never had one. The rigidity is
// real only inside the backlog system, where it is the point.
//
// So what was missing was never permission to start lower down. It was
// anyone saying so, and the entry points being reachable directly rather
// than only as a consequence of seeding from a spec.
var Entries = []Entry{
	{
		Name: "analysis",
		When: "you do not yet know what should be built, or why this and not something else",
		Next: "procoder analyze brief <name>",
	},
	{
		Name: "spec",
		When: "you know what to build and the shape is worth agreeing before code exists",
		Next: "procoder spec template <name>",
	},
	{
		Name: "plan",
		When: "the what is settled and the how is the part with risk in it",
		Next: "procoder plan new <name>",
	},
	{
		Name: "backlog",
		When: "the work is understood and needs splitting so several people or sessions can carry it",
		Next: "procoder backlog epic <title>",
	},
	{
		Name: "todo",
		When: "one bounded piece of work, understood, that nobody needs to agree about first",
		Next: "procoder todo add <title>",
	},
	{
		Name: "build",
		When: "a fix or a change small enough that writing it down costs more than doing it",
		Next: "just make the change — the gate still runs, and always did",
	},
}

// Where prints the entry points and what each is for.
//
// It is a report, not a decision: procoder cannot know how large a change
// is until it exists, and a tool that guessed would be wrong in the
// expensive direction — sending somebody to write a spec for a typo, or
// waving through a redesign because the diff started small.
func Where(root string, out func(string)) int {
	out("where to start — the chain is entered at the point that fits, not always at the top")
	out("")
	for _, e := range Entries {
		out(fmt.Sprintf("  %-9s %s", e.Name, e.When))
		out(fmt.Sprintf("  %-9s   → %s", "", e.Next))
		out("")
	}
	out("Nothing requires you to start above the point you need. No gate finding")
	out("asks for a spec, and a change that begins at build is still gated, tested,")
	out("formatted and released like every other. What the chain refuses is a story")
	out("that closes without evidence — and that refusal applies wherever you began.")

	// A repository mid-flight is told where it already is, so the advice
	// lands against its own state rather than in the abstract.
	if at := furthest(root); at != "" {
		out("")
		out("this repository has already entered at: " + at)
	}
	return 0
}

// furthest names the deepest point this repository has artifacts for —
// the entry it has in fact used, which is usually the honest answer to
// "where should this go" for the next change too.
func furthest(root string) string {
	for _, probe := range []struct{ dir, name string }{
		{".procoder/backlog/sprints", "sprint"},
		{".procoder/backlog/epics", "backlog"},
		{".procoder/plans", "plan"},
		{".procoder/specs", "spec"},
		{Dir, "analysis"},
	} {
		names, err := store.ListDir(root, probe.dir)
		if err != nil {
			continue
		}
		for _, name := range names {
			if filepath.Ext(name) == ".md" {
				return probe.name
			}
		}
	}
	return ""
}
