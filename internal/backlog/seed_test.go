package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"procoder/internal/spec"
)

// writeSpec plants a spec file under .procoder/specs/ so Seed and
// spec.Check can find it.
func writeSpec(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, spec.Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name+".md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// completeSpec builds a spec body that spec.Check accepts as COMPLETE, with
// the given lines as its Acceptance criteria section body.
func completeSpec(name, criteria string) string {
	return "# " + name + `

Status: draft

## Problem

Something concrete hurts today and this spec fixes it.

## Users

- Pascal, who runs the tool.

## In scope

- The one buildable thing.

## Out of scope

- Everything else, written down.

## Constraints

- Pure Go stdlib.

## Interfaces

- One command with one flag.

## Data

- One Markdown file under .procoder/.

## Edge cases

- The empty input refuses with a message.

## Failure modes

- A missing dependency surfaces as an error.

## Acceptance criteria

` + criteria + `

## Open questions

`
}

// mustBeComplete proves a fixture actually passes the spec controller —
// otherwise the seeding tests would be exercising the wrong refusal.
func mustBeComplete(t *testing.T, root, name string) {
	t.Helper()
	out, lines := collect()
	if code := spec.Check(root, name, out); code != 0 {
		t.Fatalf("fixture spec must be COMPLETE, got exit %d:\n%s", code, strings.Join(*lines, "\n"))
	}
}

func TestSeedRefusesIncompleteSpecAndReplaysGaps(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "half", "# half\n\n## Problem\n\nOnly a problem, nothing else.\n")
	out, lines := collect()
	if code := Seed(root, "half", "", out); code != 1 {
		t.Fatalf("incomplete spec must refuse with exit 1, got %d: %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "spec half is not COMPLETE") {
		t.Fatalf("refusal must name the spec and the rule: %v", *lines)
	}
	if !strings.Contains(joined, "section missing") {
		t.Fatalf("the checker's gap lines must be replayed: %v", *lines)
	}
}

func TestSeedRefusesPlaceholderOnlyCriteria(t *testing.T) {
	root := t.TempDir()
	// A spec whose only criterion is the template placeholder is never
	// COMPLETE, so the refusal comes from the spec controller — Seed
	// replays it rather than seeding hollow stories.
	writeSpec(t, root, "hollow", completeSpec("hollow", "- [ ] ..."))
	out, lines := collect()
	if code := Seed(root, "hollow", "", out); code != 1 {
		t.Fatalf("placeholder-only criteria must refuse with exit 1, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "placeholder") {
		t.Fatalf("the placeholder gap must be replayed: %v", *lines)
	}
}

func TestSeedRefusesSpecWithNoRealCriteria(t *testing.T) {
	root := t.TempDir()
	// The checker scans raw text, so a checkbox inside an HTML comment
	// satisfies it — but a commented-out criterion is not a story. This is
	// exactly the divergence the zero-criteria guard exists for.
	body := completeSpec("ghost", "Criteria live in the comment below.\n\n<!--\n- [ ] a criterion nobody uncommented\n-->")
	writeSpec(t, root, "ghost", body)
	mustBeComplete(t, root, "ghost")
	out, lines := collect()
	if code := Seed(root, "ghost", "", out); code != 1 {
		t.Fatalf("zero real criteria must refuse with exit 1, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "an epic with no stories is not a decomposition") {
		t.Fatalf("refusal must state the rule: %v", *lines)
	}
}

func TestSeedPrintsEpicAndOneStoryPerCriterion(t *testing.T) {
	root := t.TempDir()
	body := completeSpec("auth",
		"- [ ] login accepts a valid token and rejects an expired one\n"+
			"- [x] logout clears the session cookie on every platform\n"+
			"- [ ] the audit log records each login with a UTC\n"+
			"      timestamp")
	specPath := writeSpec(t, root, "auth", body)
	mustBeComplete(t, root, "auth")

	out, lines := collect()
	if code := Seed(root, "auth", "v1", out); code != 0 {
		t.Fatalf("seed: exit %d\n%s", code, strings.Join(*lines, "\n"))
	}
	joined := strings.Join(*lines, "\n")

	// The epic comes first, carrying spec name, fingerprint, milestone.
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	fp := fingerprint(raw)
	if len(fp) != 12 {
		t.Fatalf("fingerprint must be 12 hex chars, got %q", fp)
	}
	if !strings.Contains(joined, "== write this to "+Dir+"/epics/auth.md:") {
		t.Fatalf("epic path header missing:\n%s", joined)
	}
	if !strings.Contains(joined, "Spec: auth @ "+fp) {
		t.Fatalf("epic must record the spec fingerprint %q:\n%s", fp, joined)
	}
	if !strings.Contains(joined, "Milestone: v1") {
		t.Fatalf("epic must carry the milestone:\n%s", joined)
	}
	if !strings.Contains(joined, "Seeded from "+filepath.ToSlash(filepath.Join(spec.Dir, "auth.md"))) {
		t.Fatalf("epic description must note its origin:\n%s", joined)
	}

	// One story per criterion, each linked to the epic, id date-prefixed,
	// with its criterion replacing the template placeholder.
	if n := strings.Count(joined, "Epic: auth"); n != 3 {
		t.Fatalf("want 3 stories linked to the epic, got %d:\n%s", n, joined)
	}
	if strings.Contains(joined, "- [ ] ...") {
		t.Fatalf("no story may keep the placeholder criterion:\n%s", joined)
	}
	date := time.Now().UTC().Format("20060102")
	for _, want := range []string{
		filepath.ToSlash(filepath.Join(Dir, KindStory, date+"-login-accepts-a-valid-token-and-rejects-an-expired-one.md")),
		"- [ ] logout clears the session cookie on every platform",
		"- [ ] the audit log records each login with a UTC timestamp",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}

	// P-CONTROL: everything was printed, nothing was written.
	if _, err := os.Stat(filepath.Join(root, Dir)); !os.IsNotExist(err) {
		t.Fatal("seed must not write files")
	}
}

func TestSeedRefusesExistingEpic(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "auth", completeSpec("auth", "- [ ] login accepts a valid token"))
	mustBeComplete(t, root, "auth")
	writeItem(t, root, KindEpic, "auth", "# auth\n\nStatus: open\n")
	out, lines := collect()
	if code := Seed(root, "auth", "", out); code != 2 {
		t.Fatalf("existing epic must refuse with exit 2, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), filepath.ToSlash(filepath.Join(Dir, KindEpic, "auth.md"))) {
		t.Fatalf("refusal must name the existing epic file: %v", *lines)
	}
}
