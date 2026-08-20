package backlog

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A story with every gap the controller must find: comment-only
// description, placeholder criterion, unchecked box, comment-only
// evidence.
const hollowStory = `# Login form

Status: open
Created: 2026-08-19
Epic: auth
Sprint: -

## Description

<!-- The user story: who needs what, and why. -->

## Acceptance criteria

- [ ] ...
- [ ] the form submits

## Evidence

<!-- Filled at close time. -->
`

const solidStory = `# Login form

Status: open
Created: 2026-08-19
Epic: auth
Sprint: -

## Description

A user signs in with email and password; failures show a message.

## Acceptance criteria

- [x] the form submits
- [x] bad credentials show an error

## Evidence

go test ./web/login — both cases pass, output attached to the PR.
`

func gate(clean bool) func() bool { return func() bool { return clean } }

func TestCloseStoryWalksFromRefusedToClosed(t *testing.T) {
	root := t.TempDir()
	p := writeItem(t, root, KindStory, "20260819-login-form", hollowStory)

	// Every gap listed in ONE refusal — not just the first.
	out, lines := collect()
	if code := CloseStory(root, "20260819-login-form", gate(false), out); code != 1 {
		t.Fatalf("hollow story must refuse: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	for _, want := range []string{
		"stays OPEN",
		"a title is not a description",
		"placeholder",
		"no acceptance criterion is checked",
		"2 acceptance criterion(s) unchecked",
		"Evidence is empty",
		"the gate is not clean",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("refusal must contain %q:\n%s", want, joined)
		}
	}

	// Substance fixed, gate still dirty — the gate alone keeps it open.
	if err := os.WriteFile(p, []byte(solidStory), 0o644); err != nil {
		t.Fatal(err)
	}
	out2, lines2 := collect()
	if code := CloseStory(root, "20260819-login-form", gate(false), out2); code != 1 {
		t.Fatalf("dirty gate must refuse: exit %d %v", code, *lines2)
	}
	j2 := strings.Join(*lines2, "\n")
	if !strings.Contains(j2, "the gate is not clean") || strings.Contains(j2, "Description is empty") {
		t.Fatalf("only the gate should block now:\n%s", j2)
	}

	// Everything true — the story closes and the file records it.
	out3, lines3 := collect()
	if code := CloseStory(root, "20260819-login-form", gate(true), out3); code != 0 {
		t.Fatalf("solid story must close: exit %d %v", code, *lines3)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Status: done ") {
		t.Fatalf("close must rewrite Status: done <date>:\n%s", raw)
	}

	// Closing again is idempotent — exit 0, no rewrite complaint.
	out4, lines4 := collect()
	if code := CloseStory(root, "20260819-login-form", gate(true), out4); code != 0 {
		t.Fatalf("re-close must be idempotent: exit %d %v", code, *lines4)
	}
	if !strings.Contains(strings.Join(*lines4, "\n"), "already") {
		t.Fatalf("re-close must say already: %v", *lines4)
	}
}

func TestCloseStoryAlreadyDoneExitsZero(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindStory, "20260819-shipped", "# Shipped\n\nStatus: done 2026-01-01\nEpic: auth\nSprint: -\n")
	out, lines := collect()
	if code := CloseStory(root, "20260819-shipped", gate(false), out); code != 0 {
		t.Fatalf("already-done must exit 0: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "already done 2026-01-01") {
		t.Fatalf("must name the existing status: %v", *lines)
	}
}

func TestCloseStoryGuardsIdsAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"", "../escape", "a/b", ".."} {
		out, lines := collect()
		if code := CloseStory(root, id, gate(true), out); code != 2 {
			t.Fatalf("id %q must be refused with exit 2: %d %v", id, code, *lines)
		}
	}
	out, lines := collect()
	if code := CloseStory(root, "no-such-story", gate(true), out); code != 2 {
		t.Fatalf("missing story must exit 2: %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "procoder backlog list") {
		t.Fatalf("refusal must point at the next action: %v", *lines)
	}
}

func TestCloseEpicRefusesWhileAStoryIsOpen(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\nCreated: 2026-08-19\n\n## Description\n\nSign-in.\n")
	sp := writeItem(t, root, KindStory, "20260819-login", "# Login\n\nStatus: open\nCreated: 2026-08-19\nEpic: auth\nSprint: -\n")
	writeItem(t, root, KindStory, "20260819-other", "# Other\n\nStatus: open\nCreated: 2026-08-19\nEpic: elsewhere\nSprint: -\n")

	out, lines := collect()
	if code := CloseEpic(root, "auth", out); code != 1 {
		t.Fatalf("epic with an open story must refuse: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "20260819-login is open") {
		t.Fatalf("refusal must name the open story: %v", *lines)
	}
	if strings.Contains(joined, "20260819-other") {
		t.Fatalf("stories of other epics must not block: %v", *lines)
	}

	// The story done by hand — the epic now closes and stays closed.
	if err := os.WriteFile(sp, []byte("# Login\n\nStatus: done 2026-08-19\nEpic: auth\nSprint: -\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out2, lines2 := collect()
	if code := CloseEpic(root, "auth", out2); code != 0 {
		t.Fatalf("epic with all stories done must close: exit %d %v", code, *lines2)
	}
	raw, _ := os.ReadFile(root + "/" + Dir + "/epics/auth.md")
	if !strings.Contains(string(raw), "Status: done ") {
		t.Fatalf("epic file must record done:\n%s", raw)
	}
	out3, lines3 := collect()
	if code := CloseEpic(root, "auth", out3); code != 0 || !strings.Contains(strings.Join(*lines3, "\n"), "already") {
		t.Fatalf("re-close must be idempotent: exit %d %v", code, *lines3)
	}
}

func TestCloseEpicMissingExitsTwo(t *testing.T) {
	out, lines := collect()
	if code := CloseEpic(t.TempDir(), "ghost", out); code != 2 {
		t.Fatalf("missing epic must exit 2: %d %v", code, *lines)
	}
}

func TestCloseEpicWarnsOnSpecDrift(t *testing.T) {
	root := t.TempDir()
	specPath := root + "/.procoder/specs/auth.md"
	if err := os.MkdirAll(root+"/.procoder/specs", 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("# auth\n\nStatus: complete\n\n## Problem\n\nreal\n\n## Acceptance criteria\n\n- [ ] sign-in works\n")
	if err := os.WriteFile(specPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	// The epic records the ORIGINAL content's fingerprint at seed time.
	writeItem(t, root, KindEpic, "auth",
		"# Auth\n\nStatus: open\nCreated: 2026-08-19\nSpec: auth @ "+fingerprint(original)+"\n\n## Description\n\nSign-in.\n")
	writeItem(t, root, KindStory, "20260819-login", "# Login\n\nStatus: open\nCreated: 2026-08-19\nEpic: auth\nSprint: -\n")

	// Unchanged spec: refusal (open story) but NO drift warning.
	out, lines := collect()
	if code := CloseEpic(root, "auth", out); code != 1 {
		t.Fatalf("open story must refuse: exit %d %v", code, *lines)
	}
	if strings.Contains(strings.Join(*lines, "\n"), "⚠") {
		t.Fatalf("no drift yet, no warning: %v", *lines)
	}

	// The spec changes after seeding — the warning fires on the refusal…
	if err := os.WriteFile(specPath, []byte("# auth\n\nStatus: complete\n\n## Acceptance criteria\n\n- [ ] sign-in works\n- [ ] sign-out works\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out2, lines2 := collect()
	if code := CloseEpic(root, "auth", out2); code != 1 {
		t.Fatalf("drift must warn, never block or unblock: exit %d %v", code, *lines2)
	}
	if !strings.Contains(strings.Join(*lines2, "\n"), "⚠ spec auth changed its acceptance criteria since seeding") {
		t.Fatalf("drift warning must appear on refusal: %v", *lines2)
	}

	// …and on success, without blocking the close.
	writeItem(t, root, KindStory, "20260819-login", "# Login\n\nStatus: done 2026-08-19\nEpic: auth\nSprint: -\n")
	out3, lines3 := collect()
	if code := CloseEpic(root, "auth", out3); code != 0 {
		t.Fatalf("drift must not block the close: exit %d %v", code, *lines3)
	}
	if !strings.Contains(strings.Join(*lines3, "\n"), "⚠ spec auth changed its acceptance criteria since seeding") {
		t.Fatalf("drift warning must appear on success: %v", *lines3)
	}
}

func TestCloseEpicWarnsWhenSpecIsMissing(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth",
		"# Auth\n\nStatus: open\nCreated: 2026-08-19\nSpec: auth @ abc123def456\n\n## Description\n\nSign-in.\n")
	out, lines := collect()
	if code := CloseEpic(root, "auth", out); code != 0 {
		t.Fatalf("a missing spec warns, never blocks: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "⚠ spec auth is missing") {
		t.Fatalf("missing-spec warning must appear: %v", *lines)
	}
}

func TestCloseEpicBlocksOnUnreadableStory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 is a no-op on windows")
	}
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\nCreated: 2026-08-19\n\n## Description\n\nSign-in.\n")
	p := writeItem(t, root, KindStory, "20260819-broken", "# Broken\n\nStatus: open\nEpic: auth\n")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(p, 0o644) }()
	out, lines := collect()
	if code := CloseEpic(root, "auth", out); code != 1 {
		t.Fatalf("an unreadable story must block: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "20260819-broken is unreadable") || !strings.Contains(joined, "unknown ≠ done") {
		t.Fatalf("refusal must say unreadable and unknown ≠ done: %v", *lines)
	}
}

func TestCloseMilestoneRefusedThenClosed(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindMilestone, "v1", "# V1\n\nStatus: open\nCreated: 2026-08-19\n\n## Goal\n\nShip.\n")
	ep := writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\nCreated: 2026-08-19\nMilestone: v1\n\n## Description\n\nSign-in.\n")

	out, lines := collect()
	if code := CloseMilestone(root, "v1", out); code != 1 {
		t.Fatalf("milestone with an open epic must refuse: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "auth is open") {
		t.Fatalf("refusal must name the open epic: %v", *lines)
	}

	if err := os.WriteFile(ep, []byte("# Auth\n\nStatus: done 2026-08-19\nMilestone: v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out2, lines2 := collect()
	if code := CloseMilestone(root, "v1", out2); code != 0 {
		t.Fatalf("milestone with all epics done must close: exit %d %v", code, *lines2)
	}
	raw, _ := os.ReadFile(root + "/" + Dir + "/milestones/v1.md")
	if !strings.Contains(string(raw), "Status: done ") {
		t.Fatalf("milestone file must record done:\n%s", raw)
	}
	out3, lines3 := collect()
	if code := CloseMilestone(root, "v1", out3); code != 0 || !strings.Contains(strings.Join(*lines3, "\n"), "already") {
		t.Fatalf("re-close must be idempotent: exit %d %v", code, *lines3)
	}

	out4, lines4 := collect()
	if code := CloseMilestone(root, "ghost", out4); code != 2 {
		t.Fatalf("missing milestone must exit 2: %d %v", code, *lines4)
	}
}

// The story controller consumes the same suite verdict as todo close;
// suite nil (policy off) is the delegating CloseStory path the other tests
// cover.
func TestCloseStoryWithSuiteVerdict(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindStory, "s", solidStory)
	gate := func() bool { return true }
	red := func() (bool, string) { return false, "go: 1 test(s) failing" }
	out, lines := collect()
	if code := CloseStoryWith(root, "s", gate, red, out); code != 1 {
		t.Fatalf("a red suite must keep the story open: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "test suite is not green") {
		t.Fatalf("the refusal must say why: %v", *lines)
	}
	green := func() (bool, string) { return true, "go: pass" }
	out2, lines2 := collect()
	if code := CloseStoryWith(root, "s", gate, green, out2); code != 0 {
		t.Fatalf("a green suite must close: exit %d %v", code, *lines2)
	}
}

// Closing a sprint's worth of stories one at a time re-runs the gate and
// the whole suite once per story — 27 closes meant 27 identical
// verifications and thirteen minutes of waiting. The batch form verifies
// once and applies that one verdict to every story that earns it.
func TestCloseStoriesVerifiesOnceForTheWholeBatch(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"a", "b", "c"} {
		writeItem(t, root, KindStory, id, solidStory)
	}
	gateRuns, suiteRuns := 0, 0
	gate := func() bool { gateRuns++; return true }
	suite := func() (bool, string) { suiteRuns++; return true, "go: pass" }

	out, lines := collect()
	if code := CloseStories(root, []string{"a", "b", "c"}, gate, suite, out); code != 0 {
		t.Fatalf("all three must close: exit %d\n%s", code, strings.Join(*lines, "\n"))
	}
	if gateRuns != 1 || suiteRuns != 1 {
		t.Fatalf("the batch must verify once, got gate=%d suite=%d", gateRuns, suiteRuns)
	}
	for _, id := range []string{"a", "b", "c"} {
		raw, err := os.ReadFile(filepath.Join(root, Dir, KindStory, id+".md"))
		if err != nil || !strings.Contains(string(raw), "Status: done") {
			t.Fatalf("story %s was not closed on disk", id)
		}
	}
}

// One incomplete story must not cost the others their close, and it must
// be refused by name — the batch is a convenience, never a loosening.
func TestCloseStoriesRefusesTheIncompleteOneByName(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindStory, "good", solidStory)
	writeItem(t, root, KindStory, "hollow", hollowStory)
	gate := func() bool { return true }

	out, lines := collect()
	code := CloseStories(root, []string{"good", "hollow"}, gate, nil, out)
	joined := strings.Join(*lines, "\n")
	if code != 1 {
		t.Fatalf("a refused story makes the batch exit 1, got %d\n%s", code, joined)
	}
	if !strings.Contains(joined, "hollow") {
		t.Fatalf("the refusal must name the story: %s", joined)
	}
	raw, _ := os.ReadFile(filepath.Join(root, Dir, KindStory, "good.md"))
	if !strings.Contains(string(raw), "Status: done") {
		t.Fatal("the complete story must still close")
	}
	raw, _ = os.ReadFile(filepath.Join(root, Dir, KindStory, "hollow.md"))
	if strings.Contains(string(raw), "Status: done") {
		t.Fatal("the incomplete story must stay open")
	}
}

// The single-id form is the batch of one: same verdict, same output.
func TestCloseStoriesOfOneMatchesTheSingleForm(t *testing.T) {
	gate := func() bool { return true }
	rootA := t.TempDir()
	writeItem(t, rootA, KindStory, "s", solidStory)
	outA, linesA := collect()
	codeA := CloseStoryWith(rootA, "s", gate, nil, outA)

	rootB := t.TempDir()
	writeItem(t, rootB, KindStory, "s", solidStory)
	outB, linesB := collect()
	codeB := CloseStories(rootB, []string{"s"}, gate, nil, outB)

	if codeA != codeB || strings.Join(*linesA, "\n") != strings.Join(*linesB, "\n") {
		t.Fatalf("the batch of one must behave exactly as the single form:\n%d %v\n%d %v",
			codeA, *linesA, codeB, *linesB)
	}
}
