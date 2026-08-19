package backlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBugPrintsTypedStoryWithRegressionCriterion(t *testing.T) {
	root := t.TempDir()
	out, lines := collect()
	if code := Bug(root, "login 500s", "auth", "s1", out); code != 0 {
		t.Fatalf("bug: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "stories/") || !strings.Contains(joined, "-login-500s.md") {
		t.Fatalf("a bug is a story — date-prefixed id in the stories dir: %v", *lines)
	}
	for _, want := range []string{
		"Type: bug",
		"Severity: s1",
		"Epic: auth",
		"Sprint: -",
		"Reproduction steps",
		"Observed vs expected",
		"- [ ] a regression test pins the fix: red before the change, green after",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("bug story must contain %q:\n%s", want, joined)
		}
	}
	// P-CONTROL: printed, not written
	if _, err := os.Stat(filepath.Join(root, Dir)); !os.IsNotExist(err) {
		t.Fatal("bug creation must not write files")
	}
}

func TestBugDefaultsToS3AndNoEpic(t *testing.T) {
	out, lines := collect()
	if code := Bug(t.TempDir(), "flaky logout", "", "", out); code != 0 {
		t.Fatalf("bug: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "Severity: s3") {
		t.Fatalf("severity must default to s3: %v", *lines)
	}
	if !strings.Contains(joined, "Epic: -") {
		t.Fatalf("a bug without an epic writes Epic: - for a uniform header: %v", *lines)
	}
}

func TestBugRefusesInvalidSeverity(t *testing.T) {
	out, lines := collect()
	if code := Bug(t.TempDir(), "login 500s", "", "critical", out); code != 2 {
		t.Fatalf("invalid severity must be usage error: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "s1|s2|s3|s4") {
		t.Fatalf("refusal must name the four valid values: %v", *lines)
	}
}

// solidBug is solidStory as a defect: substance complete, but the
// Severity header is the variable each close test sets.
const solidBug = `# Login 500s

Status: open
Created: 2026-08-19
Epic: -
Sprint: -
Type: bug
%s
## Description

POST /login returns 500 when the password holds a percent sign.

## Acceptance criteria

- [x] a regression test pins the fix: red before the change, green after

## Evidence

go test ./web/login — the new regression test failed on main, passes here.
`

func TestCloseBugRefusesWithoutSeverity(t *testing.T) {
	root := t.TempDir()
	p := writeItem(t, root, KindStory, "20260819-login-500s",
		strings.Replace(solidBug, "%s\n", "", 1))
	out, lines := collect()
	if code := CloseStory(root, "20260819-login-500s", gate(true), out); code != 1 {
		t.Fatalf("a bug without a severity must stay open: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "add Severity: s1..s4") {
		t.Fatalf("refusal must name the missing header and the fix: %v", *lines)
	}

	// a hand-edited nonsense severity is the same gap, never accepted
	if err := os.WriteFile(p, []byte(strings.Replace(solidBug, "%s", "Severity: urgent", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out2, lines2 := collect()
	if code := CloseStory(root, "20260819-login-500s", gate(true), out2); code != 1 {
		t.Fatalf("an invalid severity must stay open: exit %d %v", code, *lines2)
	}

	// a real severity closes it — the type adds one check, not a new lifecycle
	if err := os.WriteFile(p, []byte(strings.Replace(solidBug, "%s", "Severity: s2", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out3, lines3 := collect()
	if code := CloseStory(root, "20260819-login-500s", gate(true), out3); code != 0 {
		t.Fatalf("a triaged bug must close: exit %d %v", code, *lines3)
	}
}

func TestBoardMarksBugsAndCountsThem(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\n")
	writeItem(t, root, KindStory, "20260819-crash",
		"# Crash\n\nStatus: open\nEpic: auth\nSprint: -\nType: bug\nSeverity: s2\n")
	writeItem(t, root, KindStory, "20260819-untriaged",
		"# Untriaged\n\nStatus: open\nEpic: -\nSprint: -\nType: bug\n")
	writeItem(t, root, KindStory, "20260819-fixed",
		"# Fixed\n\nStatus: done 2026-08-19\nEpic: auth\nSprint: -\nType: bug\nSeverity: s4\n")
	writeItem(t, root, KindStory, "20260819-feature",
		"# Feature\n\nStatus: open\nEpic: auth\nSprint: -\n")

	out, lines := collect()
	if code := Board(root, out); code != 0 {
		t.Fatalf("board: exit %d %v", code, *lines)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "[ ] B/s2 20260819-crash  Crash") {
		t.Fatalf("an open bug shows its severity marker:\n%s", joined)
	}
	if !strings.Contains(joined, "[ ] B/s? 20260819-untriaged  Untriaged") {
		t.Fatalf("a missing severity shows s?, never hidden:\n%s", joined)
	}
	if !strings.Contains(joined, "[x] B/s4 20260819-fixed  Fixed") {
		t.Fatalf("a done bug keeps its marker:\n%s", joined)
	}
	if !strings.Contains(joined, "[ ] 20260819-feature  Feature") {
		t.Fatalf("a feature story stays unmarked:\n%s", joined)
	}
	// Epic: - is no-epic, not a broken link to an epic named "-"
	if strings.Contains(joined, "epic - missing") {
		t.Fatalf("Epic: - must read as no epic, not a missing one:\n%s", joined)
	}
	if !strings.Contains(joined, "— no epic") {
		t.Fatalf("the epic-less bug still surfaces on the board:\n%s", joined)
	}
	// two of the three bugs are open; the summary says so
	if !strings.Contains(joined, "3 open · 1 done · 0 unreadable stories — active sprint: none · 2 open bug(s)") {
		t.Fatalf("summary must count open bugs separately:\n%s", joined)
	}
}

func TestBoardSummaryOmitsBugCountWhenNoneOpen(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindEpic, "auth", "# Auth\n\nStatus: open\n")
	writeItem(t, root, KindStory, "20260819-feature",
		"# Feature\n\nStatus: open\nEpic: auth\nSprint: -\n")
	out, lines := collect()
	if code := Board(root, out); code != 0 {
		t.Fatalf("board: exit %d %v", code, *lines)
	}
	if strings.Contains(strings.Join(*lines, "\n"), "open bug") {
		t.Fatalf("zero open bugs earn no count: %v", *lines)
	}
}

func TestListShowsKindBug(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, KindStory, "20260819-crash",
		"# Crash\n\nStatus: open\nEpic: -\nSprint: -\nType: bug\nSeverity: s2\n")
	out, lines := collect()
	if code := List(root, out); code != 0 {
		t.Fatalf("list: exit %d %v", code, *lines)
	}
	if (*lines)[0] != "  [open]  bug  20260819-crash  Crash" {
		t.Fatalf("list must show kind bug: %q", (*lines)[0])
	}
}
