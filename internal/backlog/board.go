// board.go is the reading side of the backlog: List is the flat
// inventory, Board is the tree — milestones over epics over stories, with
// orphans and spec drift surfaced rather than smoothed over. Both only
// read; the map must never alter the territory (P-CONTROL).

package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"procoder/internal/ask"
	"sort"
	"strings"

	"procoder/internal/gitx"
	"procoder/internal/spec"
	"procoder/internal/store"
)

// emptyHint is the one line an empty backlog prints — an instruction to
// start one, not an apology.
const emptyHint = "no backlog — `procoder backlog milestone <title>` or `procoder backlog seed <spec>` starts one"

// singular maps a kind's directory name to the word a human reads.
var singular = map[string]string{
	KindMilestone: "milestone",
	KindEpic:      "epic",
	KindStory:     "story",
	KindSprint:    "sprint",
}

// List prints every item on one line, open work before done work so the
// live backlog reads first. Statuses print verbatim — a hand-edited value
// is shown, never normalised away.
func List(root string, out func(string)) int {
	items, err := LoadAll(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if len(items) == 0 {
		out(emptyHint)
		return 0
	}
	// open, active, and anything unrecognised sort before done —
	// LoadAll's kind-then-id order survives within each half.
	sort.SliceStable(items, func(a, b int) bool {
		return !items[a].Done() && items[b].Done()
	})
	for _, it := range items {
		out(fmt.Sprintf("  [%s]  %s  %s  %s", it.Status, kindWord(it), it.ID, it.Title))
	}
	return 0
}

// Board prints the tree: each milestone with its epics nested, each epic
// with its stories, then whatever the hierarchy cannot place — epics with
// no live milestone under "(no milestone)", stories whose epic is gone
// under ORPHANS — and one summary line. Broken links are findings the
// board shows, never rows it drops.
func Board(root string, out func(string)) int {
	items, err := LoadAll(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if len(items) == 0 {
		out(emptyHint)
		return 0
	}
	// Decisions first, because a decision is what is holding other work
	// up, and a board that lists the work without the reason reads as
	// nobody having started (#191).
	printDecisions(root, items, out)

	var milestones, epics, stories []Item
	active := ""
	for _, it := range items {
		switch it.Kind {
		case KindMilestone:
			milestones = append(milestones, it)
		case KindEpic:
			epics = append(epics, it)
		case KindStory:
			stories = append(stories, it)
		case KindSprint:
			if it.Status == "active" && active == "" {
				active = it.ID
			}
		}
	}
	milestoneIDs := map[string]bool{}
	for _, m := range milestones {
		milestoneIDs[m.ID] = true
	}
	epicIDs := map[string]bool{}
	for _, e := range epics {
		epicIDs[e.ID] = true
	}
	printEpic := func(e Item) {
		out(fmt.Sprintf("  EPIC %s — %s  [%s]%s", e.ID, e.Title, e.Status, driftFlag(root, e)))
		for _, s := range stories {
			if s.Epic == e.ID {
				out(storyLine(s, ""))
			}
		}
	}
	for _, m := range milestones {
		out(fmt.Sprintf("MILESTONE %s — %s  [%s]", m.ID, m.Title, m.Status))
		for _, e := range epics {
			if e.Milestone == m.ID {
				printEpic(e)
			}
		}
	}
	// epics whose milestone is unset or hand-deleted still belong on the map
	headed := false
	for _, e := range epics {
		if e.Milestone == "" || !milestoneIDs[e.Milestone] {
			if !headed {
				out("(no milestone)")
				headed = true
			}
			printEpic(e)
		}
	}
	headed = false
	for _, s := range stories {
		if !epicIDs[s.Epic] {
			if !headed {
				out("ORPHANS")
				headed = true
			}
			note := "  — epic " + s.Epic + " missing"
			if s.Epic == "" {
				note = "  — no epic"
			}
			out(storyLine(s, note))
		}
	}
	var open, done, unreadable, openBugs int
	for _, s := range stories {
		switch {
		case s.Status == "unreadable":
			unreadable++
		case s.Done():
			done++
		default:
			open++
			if s.Type == "bug" {
				openBugs++
			}
		}
	}
	if active == "" {
		active = "none"
	}
	out("")
	summary := fmt.Sprintf("%d open · %d done · %d unreadable stories — active sprint: %s", open, done, unreadable, active)
	if openBugs > 0 {
		// open defects earn their own count — they are the work that
		// jumped the queue, and the summary must not average them away
		summary += fmt.Sprintf(" · %d open bug(s)", openBugs)
	}
	out(summary)
	if note := branchNote(root); note != "" {
		out(note)
	}
	return 0
}

// branchNote says which branch the board just read, and what the default
// branch holds that this one cannot see.
//
// The backlog is versioned like the code, so the board answers about the
// checkout while being read as answering about the project: this repository's
// own board reported "0 open · 78 done" on a feature branch while thirty-four
// specced stories sat one branch away. Naming the branch is the cheap half of
// the fix; counting what is invisible is the half that sends the reader
// somewhere. Merging the two branches' items is NOT attempted — deciding
// whose status wins when both carry a story is a design question, and a
// number that points at the answer beats a merge that guesses at it.
func branchNote(root string) string {
	here := gitx.CurrentBranch(root)
	if here == "" {
		// Not a repository, or a detached HEAD: either way there is no branch
		// to name and nothing to compare against.
		return ""
	}
	def := gitx.DefaultBranch(root)
	if def == "" {
		return "read from branch " + here + " — the default branch is unknown, so nothing was compared"
	}
	if def == here {
		return "read from branch " + here
	}
	// The remote-tracking ref, when there is one: the workflow this footer
	// exists for is fetch-and-branch, never checking the default branch out,
	// which leaves the local head behind and reading it would be the same
	// lie in a new costume.
	ref := def
	if remote := "origin/" + def; gitx.HasRef(root, remote) {
		ref = remote
	}
	theirs, err := gitx.GrepOn(root, ref, "^Status: open", filepath.ToSlash(filepath.Join(Dir, KindStory)))
	if err != nil {
		return "read from branch " + here + " — " + ref + " NOT compared: " + err.Error()
	}
	unseen := 0
	for _, rel := range theirs {
		// Any stat failure counts as unseen: a path this checkout cannot
		// look at is not one it can see, and under-counting here would put
		// the footer back to reassuring the reader wrongly.
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			unseen++
		}
	}
	if unseen == 0 {
		return "read from branch " + here + " — nothing open on " + ref + " that this branch cannot see"
	}
	return fmt.Sprintf("read from branch %s — %s has %d open story(ies) this branch cannot see; merge %s to work on them",
		here, ref, unseen, def)
}

// kindWord is the word a human reads for an item: a story typed bug reads
// as a bug, everything else keeps its kind's singular.
func kindWord(it Item) string {
	if it.Kind == KindStory && it.Type == "bug" {
		return "bug"
	}
	return singular[it.Kind]
}

// storyLine renders one story: [x] done, [ ] open (or anything the
// controller would treat as open), [!] unreadable, plus the sprint tag
// when the story is committed and any note the caller appends. A bug
// carries a B/severity marker so open s1/s2 defects read at a glance —
// s? when the header is missing, a gap shown rather than smoothed over.
func storyLine(s Item, note string) string {
	mark := " "
	switch {
	case s.Status == "unreadable":
		mark = "!"
	case s.Done():
		mark = "x"
	}
	bug := ""
	if s.Type == "bug" {
		sev := s.Severity
		if !validSeverity(sev) {
			sev = "s?"
		}
		bug = "B/" + sev + " "
	}
	tag := ""
	if s.Sprint != "" && s.Sprint != "-" {
		tag = "  → sprint " + s.Sprint
	}
	// A story missing the sections the close controller reads cannot be
	// judged at all, and that used to surface one story at a time, at close.
	// A done story is not warned about: whatever shape it closed in, the
	// controller already accepted it.
	shape := ""
	if len(s.Missing) > 0 && !s.Done() && s.Status != "unreadable" {
		shape = "  ⚠ not a story yet: no " + strings.Join(s.Missing, ", no ")
	}
	return fmt.Sprintf("    [%s] %s%s  %s%s%s%s", mark, bug, s.ID, s.Title, tag, note, shape)
}

// driftFlag compares an epic's recorded spec fingerprint against the spec
// file on disk. A changed or missing spec warns — the fingerprint is
// traceability, never a blocker; an epic without a Spec reference has
// nothing to drift from.
func driftFlag(root string, e Item) string {
	if e.SpecName == "" {
		return ""
	}
	raw, err := store.LoadIn(root, spec.Dir, e.SpecName+".md")
	if err != nil {
		return "  ⚠ spec missing"
	}
	if !recordedFingerprint.MatchString(e.SpecPrint) {
		return "  ⚠ spec not seeded"
	}
	if fingerprint(raw) != e.SpecPrint {
		return "  ⚠ spec drift"
	}
	return ""
}

// printDecisions shows what is waiting on a person, and which stories say
// they are waiting on it.
//
// The decisions themselves live in .procoder/ask/decisions.md — the
// agent's write path, and the thing it reaches for mid-work. This does not
// move them onto the backlog; it makes the backlog show that they exist,
// which is the difference between a blocked story and an unstarted one.
func printDecisions(root string, items []Item, out func(string)) {
	pending, notes, err := ask.PendingDecisions(root)
	if err != nil || len(notes) > 0 {
		// Unreadable is not "no decisions": saying nothing is waiting,
		// because procoder could not look, is the failure this whole
		// repository keeps finding.
		out("DECISIONS NOT read — " + firstProblem(err, notes))
		out("")
		return
	}
	if len(pending) == 0 {
		return
	}
	out(fmt.Sprintf("DECISIONS WAITING (%d) — work below may be held up by these", len(pending)))
	for _, q := range pending {
		head := firstLineOf(q.Text)
		out("  ? " + head)
		for _, it := range items {
			if it.BlockedBy != "" && mentions(head, it.BlockedBy) {
				out("      blocks " + it.ID)
			}
		}
	}
	out("")
}

func firstProblem(err error, notes []string) string {
	if err != nil {
		return err.Error()
	}
	if len(notes) > 0 {
		return notes[0]
	}
	return "reason unknown"
}

func firstLineOf(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// mentions matches a story's Blocked-by: against a decision's heading
// loosely, because a person writing the header will shorten the question
// rather than paste it.
func mentions(decision, blockedBy string) bool {
	d, b := strings.ToLower(decision), strings.ToLower(strings.TrimSpace(blockedBy))
	return b != "" && (strings.Contains(d, b) || strings.Contains(b, d))
}
