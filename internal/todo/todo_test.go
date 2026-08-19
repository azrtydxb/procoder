package todo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTask(t *testing.T, root, id, content string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

const completeTask = `# Ship the widget

Status: open
Created: 2026-08-19

## Description

Build the widget so the report renders offline.

## Acceptance criteria

- [x] widget renders with no network
- [x] go test ./... passes

## Evidence

Ran go test ./... — all green. Loaded the report with wifi off; rendered.
`

func TestCloseRefusesUntilComplete(t *testing.T) {
	gateClean := func() bool { return true }

	cases := []struct {
		name, content, wantMiss string
	}{
		{"empty description", strings.Replace(completeTask,
			"Build the widget so the report renders offline.", "", 1),
			"Description is empty"},
		{"unchecked criterion", strings.Replace(completeTask,
			"- [x] widget renders with no network", "- [ ] widget renders with no network", 1),
			"unchecked"},
		{"placeholder criteria", strings.Replace(completeTask,
			"- [x] widget renders with no network", "- [ ] ...", 1),
			"placeholder"},
		{"empty evidence", strings.Replace(completeTask,
			"Ran go test ./... — all green. Loaded the report with wifi off; rendered.", "", 1),
			"Evidence is empty"},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			id := fmt.Sprintf("task-%d", i)
			writeTask(t, root, id, c.content)
			out, lines := collect()
			if code := Close(root, id, gateClean, out); code != 1 {
				t.Fatalf("exit %d, want 1 (refused); output: %v", code, *lines)
			}
			joined := strings.Join(*lines, "\n")
			if !strings.Contains(joined, c.wantMiss) {
				t.Errorf("refusal does not name %q:\n%s", c.wantMiss, joined)
			}
			raw, _ := os.ReadFile(filepath.Join(root, Dir, id+".md"))
			if !strings.Contains(string(raw), "Status: open") {
				t.Error("a refused task must stay open on disk")
			}
		})
	}
}

func TestCloseRefusesWhenGateDirty(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, "t", completeTask)
	out, lines := collect()
	gateRan := false
	if code := Close(root, "t", func() bool { gateRan = true; return false }, out); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !gateRan {
		t.Error("the gate was never consulted")
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "gate is not clean") {
		t.Errorf("refusal does not name the gate: %v", *lines)
	}
}

func TestCloseClosesWhenComplete(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, "t", completeTask)
	out, _ := collect()
	if code := Close(root, "t", func() bool { return true }, out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	raw, _ := os.ReadFile(filepath.Join(root, Dir, "t.md"))
	if !strings.Contains(string(raw), "Status: closed") {
		t.Error("task not marked closed on disk")
	}
	// closing again is a no-op, not an error
	out2, lines := collect()
	if code := Close(root, "t", func() bool { return true }, out2); code != 0 {
		t.Fatalf("re-close exit %d, want 0; %v", code, *lines)
	}
}

func TestCloseMissingTask(t *testing.T) {
	out, _ := collect()
	if code := Close(t.TempDir(), "nope", func() bool { return true }, out); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestCloseRefusesTraversalIDs(t *testing.T) {
	root := t.TempDir()
	// a file OUTSIDE the todo dir that a traversal id would reach
	if err := os.WriteFile(filepath.Join(root, "escape.md"), []byte(completeTask), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../escape", "..", "a/b", "../../etc/passwd"} {
		out, _ := collect()
		if code := Close(root, id, func() bool { return true }, out); code != 2 {
			t.Errorf("id %q: exit %d, want 2 (refused)", id, code)
		}
	}
	if _, err := File(root, "fine-id"); err != nil {
		t.Errorf("plain id refused: %v", err)
	}
}

func TestListOrdersOpenFirst(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, "a-closed", "# A\n\nStatus: closed\n")
	writeTask(t, root, "b-open", "# B\n\nStatus: open\n")
	tasks, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != "b-open" || tasks[0].Title != "B" {
		t.Fatalf("unexpected order: %+v", tasks)
	}
}

func TestListNoDir(t *testing.T) {
	tasks, err := List(t.TempDir())
	if err != nil || tasks != nil {
		t.Fatalf("no todo dir must be empty and error-free, got %v %v", tasks, err)
	}
}

func TestAddEmitsTemplate(t *testing.T) {
	out, lines := collect()
	if code := Add(t.TempDir(), "Fix the flaky test", out); code != 0 {
		t.Fatalf("exit %d", code)
	}
	joined := strings.Join(*lines, "\n")
	if !strings.Contains(joined, "fix-the-flaky-test.md") {
		t.Errorf("no slugged path in output:\n%s", joined)
	}
	if !strings.Contains(joined, "## Acceptance criteria") {
		t.Errorf("template sections missing:\n%s", joined)
	}
	if code := Add(t.TempDir(), "!!!", out); code != 2 {
		t.Error("an unsluggable title must be refused")
	}
}

// Under [test] policy = "block" the suite verdict joins the controller;
// suite nil (the policy off) is the delegating Close path every other test
// already covers.
func TestCloseWithSuiteVerdict(t *testing.T) {
	root := t.TempDir()
	writeTask(t, root, "t", completeTask)
	gate := func() bool { return true }
	red := func() (bool, string) { return false, "go: 2 test(s) failing" }
	out, lines := collect()
	if code := CloseWith(root, "t", gate, red, out); code != 1 {
		t.Fatalf("a red suite must keep the task open: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "test suite is not green — go: 2 test(s) failing") {
		t.Fatalf("the refusal must carry the suite summary: %v", *lines)
	}
	green := func() (bool, string) { return true, "go: pass" }
	out2, lines2 := collect()
	if code := CloseWith(root, "t", gate, green, out2); code != 0 {
		t.Fatalf("a green suite must close: exit %d %v", code, *lines2)
	}
}
