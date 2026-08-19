// Package lessons owns the self-learning loop's ledger: every finding that
// escaped procoder's own layers and was caught downstream (a bot reviewer,
// a human, production) becomes a lesson, and every lesson must carry the
// adaptation that closes its class — a linter enabled, a rubric line, a
// pinning test, a controller tightened. The binary reports which lessons
// are still unlearned (recorded but not adapted); the agent adapts.
package lessons

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path is the ledger, under D-HOME with the other github rules files.
const Path = ".procoder/github/LESSONS.md"

// ReviewPath is the pre-PR review rubric the lessons feed.
const ReviewPath = ".procoder/github/REVIEW.md"

// DefaultLedger is the ledger's shape, printed by templates when missing.
// The entry example is indented as a code block ON PURPOSE: Parse treats
// every line starting "## " as an entry, so a bare example heading would
// make a fresh ledger fail its own check (pinned by test).
const DefaultLedger = `# Lessons — findings that escaped our own gates

One entry per finding caught downstream (bot review, human review,
production) — the escape is the bug; the finding is its symptom. Every
entry names which layer should have caught it and the adaptation that now
does. ` + "`procoder lessons`" + ` flags entries with no adaptation.

Entry shape (unindented in real entries):

    ## <date> <where caught> — <one-line finding>

    - Class: mechanical | judgment | taste
    - Missed by: linter | rubric | controller | test | ci
    - Adaptation: <the concrete change that catches this class from now on>
`

// DefaultReview is the pre-PR self-review rubric: a fresh-context reviewer
// reads ONLY the diff against this list before a PR exists. Seeded from
// the classes bot reviewers actually caught; grown by the lessons loop.
// A repo's tracked REVIEW.md starts as this text and DIVERGES by design
// as lessons add lines — no drift guard binds the pair.
const DefaultReview = `# Pre-PR review rubric

A fresh-context reviewer (a subagent, not the author) reads the full
branch diff against this list BEFORE the PR is opened. The author fixes
Critical/Important findings first; downstream reviewers are the fallback,
not the net. Findings name file:line, what breaks, and the fix.

Check every hunk for:

- User-supplied strings reaching a path, command, or query — validated as
  the plain value they claim to be (no separators, no dot-dot, quoted)?
- Error paths: any error swallowed, any unreadable input silently
  skipped, any failure reported as success? Honesty beats convenience.
- State computed twice that must agree (time.Now called twice across a
  boundary, a value re-derived instead of passed).
- Loops doing per-iteration work that belongs outside (regex compilation,
  allocations, file opens).
- Temp files and permissions: CreateTemp over predictable names; modes no
  wider than needed.
- New surface wired everywhere it must appear: dispatch, usage text,
  canonical lists, docs, tests that pin them together.
- Parsers and scanners against hostile shapes: empty input, binary input,
  the terminator variants, the case the happy path skips.
- Test fixtures that trip our own scanners: assemble marker/secret-like
  content at runtime, never as a literal.
- Prose and markdown: code spans unbroken, lists formatted, wording that
  says what the code actually does.

End with a verdict line: findings counted by severity, or exactly
"Nothing found — open the PR."
`

// Entry is one parsed lesson.
type Entry struct {
	Title      string
	Adaptation string
}

// Parse reads the ledger's entries.
func Parse(text string) []Entry {
	var entries []Entry
	// leading newline so an entry on the file's first line still splits
	blocks := strings.Split("\n"+text, "\n## ")
	for _, b := range blocks[1:] {
		lines := strings.SplitN(b, "\n", 2)
		e := Entry{Title: strings.TrimSpace(lines[0])}
		if len(lines) > 1 {
			for _, l := range strings.Split(lines[1], "\n") {
				l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "-"))
				if rest, ok := strings.CutPrefix(l, "Adaptation:"); ok {
					e.Adaptation = strings.TrimSpace(rest)
				}
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// Run prints the ledger state and flags unlearned lessons.
func Run(root string, out func(string)) int {
	raw, err := os.ReadFile(filepath.Join(root, Path))
	if os.IsNotExist(err) {
		out("no lessons ledger — `procoder templates` prints the shape for " + Path)
		return 0
	}
	if err != nil {
		out("lessons ledger unreadable: " + err.Error())
		return 2
	}
	entries := Parse(string(raw))
	if len(entries) == 0 {
		out("lessons ledger has no entries yet — nothing has escaped, or nothing was recorded")
		return 0
	}
	unlearned := 0
	for _, e := range entries {
		if e.Adaptation == "" || strings.HasPrefix(e.Adaptation, "<") {
			unlearned++
			out("  UNLEARNED  " + e.Title + " — no adaptation recorded; the class is still open")
		} else {
			out("  learned    " + e.Title)
		}
	}
	out(fmt.Sprintf("procoder lessons: %d lesson(s), %d unlearned", len(entries), unlearned))
	if unlearned > 0 {
		return 1
	}
	return 0
}
