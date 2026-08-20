package backlog

import (
	"strings"
	"testing"
)

// boardTree builds the fixture the board tests read: one milestone, three
// epics under it (drifted spec, missing spec, matching spec), a done and
// an open story, an orphan story, and an active sprint.
func boardTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeItem(t, root, KindMilestone, "v1", "# V1\n\nStatus: open\n")
	// the auth spec was seeded at one fingerprint, then hand-modified
	seeded := []byte("# auth\n\nStatus: draft\n\n## Problem\n\noriginal\n")
	writeSpec(t, root, "auth", string(seeded)+"\nedited after seeding\n")
	writeItem(t, root, KindEpic, "auth",
		"# Auth\n\nStatus: open\nMilestone: v1\nSpec: auth @ "+fingerprint(seeded)+"\n")
	// the billing spec file was deleted after seeding
	writeItem(t, root, KindEpic, "billing",
		"# Billing\n\nStatus: open\nMilestone: v1\nSpec: billing @ abc123def456\n")
	// the search spec is untouched — its fingerprint still matches
	intact := []byte("# search\n\nStatus: draft\n")
	writeSpec(t, root, "search", string(intact))
	writeItem(t, root, KindEpic, "search",
		"# Search\n\nStatus: open\nMilestone: v1\nSpec: search @ "+fingerprint(intact)+"\n")
	writeItem(t, root, KindStory, "20260101-signup",
		"# Signup\n\nStatus: done 2026-08-01\nEpic: auth\nSprint: -\n")
	writeItem(t, root, KindStory, "20260102-login",
		"# Login\n\nStatus: open\nEpic: auth\nSprint: 001-mvp\n")
	writeItem(t, root, KindStory, "20260103-ghost",
		"# Ghost\n\nStatus: open\nEpic: gone\nSprint: -\n")
	writeItem(t, root, KindSprint, "001-mvp", "# mvp\n\nStatus: active\n")
	return root
}

func TestBoardNestsAndFlags(t *testing.T) {
	out, lines := collect()
	if code := Board(boardTree(t), out); code != 0 {
		t.Fatalf("board: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	find := func(want string) int {
		t.Helper()
		for i, l := range *lines {
			if strings.Contains(l, want) {
				return i
			}
		}
		t.Fatalf("board must contain %q:\n%s", want, joined)
		return -1
	}
	// the tree: milestone first, its epics nested beneath
	mi := find("MILESTONE v1 — V1  [open]")
	ei := find("EPIC auth — Auth  [open]")
	if ei <= mi {
		t.Fatalf("epic must nest under its milestone:\n%s", joined)
	}
	// drift flags: modified spec drifts, deleted spec is missing,
	// the untouched spec earns no flag at all
	if !strings.Contains((*lines)[ei], "⚠ spec drift") {
		t.Fatalf("modified spec must flag drift: %q", (*lines)[ei])
	}
	find("EPIC billing — Billing  [open]  ⚠ spec missing")
	si := find("EPIC search — Search")
	if strings.Contains((*lines)[si], "⚠") {
		t.Fatalf("matching fingerprint must not flag: %q", (*lines)[si])
	}
	// story marks and the sprint tag
	di := find("[x] 20260101-signup  Signup")
	li := find("[ ] 20260102-login  Login  → sprint 001-mvp")
	if di <= ei || li <= ei {
		t.Fatalf("stories must nest under their epic:\n%s", joined)
	}
	// the orphan surfaces with its broken link named
	oi := find("ORPHANS")
	gi := find("[ ] 20260103-ghost  Ghost  — epic gone missing")
	if gi <= oi {
		t.Fatalf("orphan story must sit under the ORPHANS heading:\n%s", joined)
	}
	// the summary: signup done, login and ghost open, sprint named
	find("2 open · 1 done · 0 unreadable stories — active sprint: 001-mvp")
}

func TestBoardAndListEmptyBacklog(t *testing.T) {
	for name, fn := range map[string]func(string, func(string)) int{
		"list": List, "board": Board,
	} {
		out, lines := collect()
		if code := fn(t.TempDir(), out); code != 0 {
			t.Fatalf("%s on empty backlog: exit %d %v", name, code, *lines)
		}
		if len(*lines) != 1 || !strings.Contains((*lines)[0], "no backlog") ||
			!strings.Contains((*lines)[0], "procoder backlog milestone") {
			t.Fatalf("%s must print the one starter line: %v", name, *lines)
		}
	}
}

func TestListOrdersOpenBeforeDone(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindMilestone, "v0", "# V0\n\nStatus: done 2026-01-01\n")
	writeItem(t, root, KindStory, "20260102-login",
		"# Login\n\nStatus: open\nEpic: auth\nSprint: -\n")
	out, lines := collect()
	if code := List(root, out); code != 0 {
		t.Fatalf("list: exit %d %v", code, *lines)
	}
	if len(*lines) != 2 {
		t.Fatalf("want 2 lines, got %v", *lines)
	}
	if (*lines)[0] != "  [open]  story  20260102-login  Login" {
		t.Fatalf("open item must lead, in the exact shape: %q", (*lines)[0])
	}
	if (*lines)[1] != "  [done 2026-01-01]  milestone  v0  V0" {
		t.Fatalf("done item must trail, status verbatim: %q", (*lines)[1])
	}
}

// An epic whose Spec: line carries something no fingerprint could produce was
// never seeded from that spec — the binary prints the epic and the agent
// writes it, so a placeholder can land where the digest belongs. That is a
// different fact from a spec that changed, and reporting it as drift sends
// the reader to compare a spec against a seeding that never happened.
// proved by: compared the digest without checking its shape — a hand-written
// `@ 0` then reads as drift, permanently, and no re-seeding can clear it.
func TestAnEpicThatWasNeverSeededSaysSoInsteadOfClaimingDrift(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "auth", "# auth\n\nthe spec\n")
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open 2026-08-20\nSpec: auth @ 0\n")

	var lines []string
	if code := Board(root, func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("board exit code = %d, want 0", code)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "⚠ spec not seeded") {
		t.Errorf("a fingerprint that was never recorded must say so: %s", joined)
	}
	if strings.Contains(joined, "⚠ spec drift") {
		t.Errorf("nothing drifted — there was never a seeding to drift from: %s", joined)
	}
}
