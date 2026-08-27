package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func blockedRepo(t *testing.T, decisions, blockedBy string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{".procoder/ask", ".procoder/backlog/epics", ".procoder/backlog/stories"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".procoder/ask/decisions.md", decisions)
	write(".procoder/backlog/epics/e.md", "# e\n\nStatus: open\nCreated: 2026-08-27\n")
	story := "# A story\n\nStatus: open\nCreated: 2026-08-27\nEpic: e\n"
	if blockedBy != "" {
		story += "Blocked-by: " + blockedBy + "\n"
	}
	story += "\n## Description\n\nx\n\n## Acceptance criteria\n\n- [ ] `procoder check` passes; fails if not.\n\n## Evidence\n"
	write(".procoder/backlog/stories/s1.md", story)
	return root
}

func lines(t *testing.T, root string) string {
	t.Helper()
	var out []string
	Board(root, func(s string) { out = append(out, s) })
	return strings.Join(out, "\n")
}

// The whole of #191: a story blocked by an undecided question looked
// exactly like a story nobody had started.
//
// proved by: the printDecisions call removed from Board — the decision
// vanishes and the blocked story is indistinguishable from an unstarted
// one again.
func TestABlockedStoryShowsWhatBlocksIt(t *testing.T) {
	root := blockedRepo(t, "## Should the cache be shared between agents?\n\n- yes\n- no\n", "Should the cache be shared")
	got := lines(t, root)
	if !strings.Contains(got, "DECISIONS WAITING") {
		t.Fatalf("the outstanding decision was not shown:\n%s", got)
	}
	if !strings.Contains(got, "blocks s1") {
		t.Errorf("the story it blocks was not named:\n%s", got)
	}
}

// No decisions means no section — a board that announces an empty heading
// on every run trains people to skip it.
//
// proved by: the `len(pending) == 0` early return removed — the heading
// prints with nothing under it.
func TestNoDecisionsPrintsNothing(t *testing.T) {
	root := blockedRepo(t, "", "")
	if got := lines(t, root); strings.Contains(got, "DECISIONS") {
		t.Fatalf("an empty decisions file produced a heading:\n%s", got)
	}
}

// Unreadable is not "none waiting". Saying nothing is outstanding because
// procoder could not look is the shape this repository has now found six
// times.
//
// proved by: the error branch in printDecisions made to return silently —
// the board reports a clean backlog over a decisions file it could not
// read.
func TestAnUnreadableDecisionsFileIsSaidOutLoud(t *testing.T) {
	root := blockedRepo(t, "not a heading, just prose\n", "")
	got := lines(t, root)
	if !strings.Contains(got, "DECISIONS NOT read") {
		t.Fatalf("a decisions file procoder could not parse was passed over in silence:\n%s", got)
	}
}
