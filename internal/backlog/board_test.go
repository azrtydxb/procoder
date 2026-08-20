package backlog

import (
	"os"
	"os/exec"
	"path/filepath"
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
	seeded := []byte("# auth\n\nStatus: draft\n\n## Acceptance criteria\n\n- [ ] sign-in works\n")
	writeSpec(t, root, "auth", "# auth\n\nStatus: draft\n\n## Acceptance criteria\n\n- [ ] sign-in works\n- [ ] sign-out works\n")
	writeItem(t, root, KindEpic, "auth",
		"# Auth\n\nStatus: open\nMilestone: v1\nSpec: auth @ "+fingerprint(seeded)+"\n")
	// the billing spec file was deleted after seeding
	writeItem(t, root, KindEpic, "billing",
		"# Billing\n\nStatus: open\nMilestone: v1\nSpec: billing @ abc123def456\n")
	// the search spec is untouched — its fingerprint still matches
	intact := []byte("# search\n\nStatus: draft\n\n## Acceptance criteria\n\n- [ ] results rank\n")
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

// A Spec: line with no fingerprint at all is the same fact as one carrying a
// value no fingerprint could produce, and it used to be worse: the header did
// not parse, so the epic read as having no spec reference — no link on the
// board and nothing to flag. Silence is the one answer a reader cannot act on.
// proved by: required the `@ <print>` half to match — the epic then claims no
// spec at all and the board says nothing about it.
func TestAnEpicNamingASpecWithNoFingerprintIsNotSilent(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "auth", "# auth\n\nthe spec\n")
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open 2026-08-20\nSpec: auth\n")

	var lines []string
	if code := Board(root, func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("board exit code = %d, want 0", code)
	}
	if joined := strings.Join(lines, "\n"); !strings.Contains(joined, "⚠ spec not seeded") {
		t.Errorf("an epic that names a spec but records no seeding must say so: %s", joined)
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

// TestTheBoardNamesAStoryNothingCanJudge pins the earlier warning: four
// stories in sprint 006 were written with Steps/Files/Verification instead of
// the sections CloseStory reads, and nothing said so until someone tried to
// close them, one at a time. An open story missing a required section says so
// on the board; a done one does not, because the controller already accepted
// whatever shape it closed in.
func TestTheBoardNamesAStoryNothingCanJudge(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\n")
	writeItem(t, root, KindStory, "20260101-shapeless",
		"# Shapeless\n\nStatus: open\nEpic: auth\nSprint: -\n\n## Steps\n\n1. do it\n")
	writeItem(t, root, KindStory, "20260102-whole",
		"# Whole\n\nStatus: open\nEpic: auth\nSprint: -\n\n## Description\n\nreal\n\n## Acceptance criteria\n\n- [ ] it works\n\n## Evidence\n\n- none yet\n")
	writeItem(t, root, KindStory, "20260103-closed",
		"# Closed\n\nStatus: done 2026-01-03\nEpic: auth\nSprint: -\n\n## Steps\n\n1. done long ago\n")

	out, lines := collect()
	if code := Board(root, out); code != 0 {
		t.Fatalf("board: exit %d %v", code, *lines)
	}
	for _, c := range []struct{ id, want string }{
		{"20260101-shapeless", "⚠ not a story yet: no Description, no Acceptance criteria, no Evidence"},
		{"20260102-whole", ""},
		{"20260103-closed", ""},
	} {
		for _, l := range *lines {
			if !strings.Contains(l, c.id) {
				continue
			}
			switch {
			case c.want == "" && strings.Contains(l, "⚠"):
				t.Errorf("%s must earn no shape warning: %q", c.id, l)
			case c.want != "" && !strings.Contains(l, c.want):
				t.Errorf("%s must name every missing section, got %q", c.id, l)
			}
		}
	}
}

// TestTheBoardSaysWhichBranchItRead pins the lie this footer exists to stop:
// the backlog is versioned like the code, so a board run on a feature branch
// reported "0 open · 78 done" while thirty-four specced stories sat one
// branch away, and it was read as a statement about the project. The footer
// names the branch and counts what the default branch holds that this
// checkout cannot see. It does not merge the two — whose status wins when
// both carry a story is a design question, not a default.
func TestTheBoardSaysWhichBranchItRead(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v\n%s", err, out)
		}
	}
	story := func(id, status string) {
		t.Helper()
		writeItem(t, root, KindStory, id, "# "+id+"\n\nStatus: "+status+"\nEpic: auth\nSprint: -\n"+
			"\n## Description\n\nreal\n\n## Acceptance criteria\n\n- [ ] it works\n\n## Evidence\n\n- none\n")
	}
	run("init", "-q", "-b", "main")
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\n")
	story("20260101-onmain", "open")
	run("add", "-A")
	run("commit", "-qm", "main has a story")

	// On main itself the footer names the branch and compares nothing.
	out, lines := collect()
	Board(root, out)
	if last := (*lines)[len(*lines)-1]; last != "read from branch main" {
		t.Errorf("on the default branch the footer just names it, got %q", last)
	}

	// A branch that never received that story cannot see it.
	run("checkout", "-q", "-b", "feature")
	if err := os.Remove(filepath.Join(root, Dir, KindStory, "20260101-onmain.md")); err != nil {
		t.Fatal(err)
	}
	story("20260102-onbranch", "open")
	run("add", "-A")
	run("commit", "-qm", "the branch goes its own way")

	out2, lines2 := collect()
	Board(root, out2)
	last := (*lines2)[len(*lines2)-1]
	if !strings.Contains(last, "read from branch feature") || !strings.Contains(last, "main has 1 open story(ies) this branch cannot see") {
		t.Errorf("the footer must count what the default branch holds and this one lacks, got %q", last)
	}
	if !strings.Contains(strings.Join(*lines2, "\n"), "1 open · 0 done") {
		t.Errorf("the counts stay about this checkout: %v", *lines2)
	}
}

// Outside a repository there are no branches to name, so the board says
// nothing rather than inventing an unknown.
func TestTheBranchFooterIsSilentOutsideARepository(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\n")
	out, lines := collect()
	Board(root, out)
	if joined := strings.Join(*lines, "\n"); strings.Contains(joined, "read from branch") {
		t.Errorf("no repository, no branch line:\n%s", joined)
	}
}

// TestTheFooterCountsAccentedPathsCorrectly pins the quoting bug the footer
// shipped with: `git grep -l` quotes any path carrying a non-ASCII byte
// (caf\303\251-story.md), that spelling exists nowhere on disk, and every
// accented story therefore stat-ed as missing — a branch with an identical
// tree was told the default branch held stories it could not see.
func TestTheFooterCountsAccentedPathsCorrectly(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git unavailable: %v\n%s", err, out)
		}
	}
	run("init", "-q", "-b", "main")
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\n")
	writeItem(t, root, KindStory, "20260101-café",
		"# Café\n\nStatus: open\nEpic: auth\nSprint: -\n\n## Description\n\nreal\n\n## Acceptance criteria\n\n- [ ] x\n\n## Evidence\n\n- y\n")
	run("add", "-A")
	run("commit", "-qm", "a story whose name needs quoting")
	run("checkout", "-q", "-b", "feature")
	run("commit", "-q", "--allow-empty", "-m", "an identical tree")

	out, lines := collect()
	Board(root, out)
	last := (*lines)[len(*lines)-1]
	if !strings.Contains(last, "nothing open on") {
		t.Errorf("an identical tree hides nothing — got %q", last)
	}
}
