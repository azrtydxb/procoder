// board.go is the reading side of the backlog: List is the flat
// inventory, Board is the tree — milestones over epics over stories, with
// orphans and spec drift surfaced rather than smoothed over. Both only
// read; the map must never alter the territory (P-CONTROL).

package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"procoder/internal/spec"
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
		out(fmt.Sprintf("  [%s]  %s  %s  %s", it.Status, singular[it.Kind], it.ID, it.Title))
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
	var open, done, unreadable int
	for _, s := range stories {
		switch {
		case s.Status == "unreadable":
			unreadable++
		case s.Done():
			done++
		default:
			open++
		}
	}
	if active == "" {
		active = "none"
	}
	out("")
	out(fmt.Sprintf("%d open · %d done · %d unreadable stories — active sprint: %s", open, done, unreadable, active))
	return 0
}

// storyLine renders one story: [x] done, [ ] open (or anything the
// controller would treat as open), [!] unreadable, plus the sprint tag
// when the story is committed and any note the caller appends.
func storyLine(s Item, note string) string {
	mark := " "
	switch {
	case s.Status == "unreadable":
		mark = "!"
	case s.Done():
		mark = "x"
	}
	tag := ""
	if s.Sprint != "" && s.Sprint != "-" {
		tag = "  → sprint " + s.Sprint
	}
	return fmt.Sprintf("    [%s] %s  %s%s%s", mark, s.ID, s.Title, tag, note)
}

// driftFlag compares an epic's recorded spec fingerprint against the spec
// file on disk. A changed or missing spec warns — the fingerprint is
// traceability, never a blocker; an epic without a Spec reference has
// nothing to drift from.
func driftFlag(root string, e Item) string {
	if e.SpecName == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(root, spec.Dir, e.SpecName+".md"))
	if err != nil {
		return "  ⚠ spec missing"
	}
	if fingerprint(raw) != e.SpecPrint {
		return "  ⚠ spec drift"
	}
	return ""
}
