package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePlan(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, Dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

const completePlan = `# widget — implementation plan

Status: draft
Spec: .procoder/specs/widget.md

## Goal

Render the report offline.

## Architecture

A cache layer in front of the fetcher; the renderer reads the cache.

## Constraints

Go 1.22 floor. No new dependencies.

## Task 1: cache layer

Files: internal/cache/cache.go (new), internal/cache/cache_test.go (new)
Interfaces: produces Get(key string) ([]byte, bool)

- [ ] write TestGetMissReturnsFalse; run it, expect FAIL with "want false"
- [ ] implement Get with a map; run the test, expect pass
- [ ] commit

## Task 2: renderer reads cache

Files: internal/render/render.go (modify)
Interfaces: consumes cache.Get(key string) ([]byte, bool)

- [ ] write TestRenderUsesCache; expect FAIL
- [ ] wire cache.Get into render; expect pass
- [ ] commit
`

func TestCheckPassesCompletePlan(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "widget", completePlan)
	out, lines := collect()
	if code := Check(root, "widget", out); code != 0 {
		t.Fatalf("exit %d, want 0:\n%s", code, strings.Join(*lines, "\n"))
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "COMPLETE") {
		t.Errorf("no COMPLETE verdict: %v", *lines)
	}
}

func TestCheckBlocksOnGaps(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		wantGap string
	}{
		{"placeholder TBD", func(s string) string {
			return strings.Replace(s, "Go 1.22 floor.", "TBD", 1)
		}, "placeholder"},
		{"similar to task", func(s string) string {
			return strings.Replace(s, "wire cache.Get into render", "similar to Task 1", 1)
		}, "placeholder"},
		{"handle edge cases", func(s string) string {
			return strings.Replace(s, "implement Get with a map", "implement Get and handle edge cases", 1)
		}, "placeholder"},
		{"task without files", func(s string) string {
			return strings.Replace(s, "Files: internal/render/render.go (modify)\n", "", 1)
		}, "Task 2 names no Files:"},
		{"empty goal", func(s string) string {
			return strings.Replace(s, "Render the report offline.", "", 1)
		}, "section empty: Goal"},
		{"no tasks", func(s string) string {
			i := strings.Index(s, "## Task 1")
			return s[:i]
		}, "no `## Task N:` blocks"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writePlan(t, root, "widget", c.mutate(completePlan))
			out, lines := collect()
			if code := Check(root, "widget", out); code != 1 {
				t.Fatalf("exit %d, want 1 (blocked):\n%s", code, strings.Join(*lines, "\n"))
			}
			if !strings.Contains(strings.Join(*lines, "\n"), c.wantGap) {
				t.Errorf("gap %q not named:\n%s", c.wantGap, strings.Join(*lines, "\n"))
			}
		})
	}
}

func TestTaskWithoutStepsBlocks(t *testing.T) {
	root := t.TempDir()
	mutated := strings.Replace(completePlan,
		`- [ ] write TestRenderUsesCache; expect FAIL
- [ ] wire cache.Get into render; expect pass
- [ ] commit
`, "", 1)
	writePlan(t, root, "widget", mutated)
	out, lines := collect()
	if code := Check(root, "widget", out); code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, strings.Join(*lines, "\n"))
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "Task 2 has no checkbox steps") {
		t.Errorf("steps gap not named:\n%s", strings.Join(*lines, "\n"))
	}
}

func TestTemplateFailsItsOwnCheck(t *testing.T) {
	// an unfilled template must block — its guidance is in comments, so
	// the sections read as empty and there are no real steps
	root := t.TempDir()
	writePlan(t, root, "fresh", strings.ReplaceAll(Template, "%s", "fresh"))
	out, lines := collect()
	if code := Check(root, "fresh", out); code != 1 {
		t.Fatalf("an unfilled template must block, exit %d:\n%s", code, strings.Join(*lines, "\n"))
	}
}

func TestCheckRefusesTraversalNames(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "widget", completePlan)
	for _, name := range []string{"../widget", "..", "a/b"} {
		out, _ := collect()
		if code := Check(root, name, out); code != 2 {
			t.Errorf("name %q: exit %d, want 2 (refused)", name, code)
		}
	}
	out, _ := collect()
	if code := PrintTemplate("../escape", out); code != 2 {
		t.Error("template must refuse traversal names")
	}
}

func TestCheckAllAndEmpty(t *testing.T) {
	root := t.TempDir()
	out, _ := collect()
	if code := Check(root, "", out); code != 0 {
		t.Fatal("no plans is not a failure")
	}
	writePlan(t, root, "good", completePlan)
	writePlan(t, root, "bad", "# bad\n\n## Goal\n\nX.\n")
	out2, _ := collect()
	if code := Check(root, "all", out2); code != 1 {
		t.Fatal("one incomplete plan must block `check all`")
	}
}

// The placeholder rule is case-sensitive for TODO on purpose: lowercase
// "todo" names procoder's own task domain, and a plan that touches
// internal/todo must be writable.
func TestLowercaseTodoIsNotAPlaceholder(t *testing.T) {
	root := t.TempDir()
	writePlan(t, root, "widget", strings.Replace(completePlan,
		"A cache layer in front of the fetcher; the renderer reads the cache.",
		"Mirrors todo.Close from the todo domain; internal/todo stays untouched.", 1))
	out, lines := collect()
	if code := Check(root, "widget", out); code != 0 {
		t.Fatalf("lowercase todo must pass: exit %d\n%s", code, strings.Join(*lines, "\n"))
	}

	writePlan(t, root, "marked", strings.Replace(completePlan,
		"A cache layer in front of the fetcher; the renderer reads the cache.",
		"TODO decide the cache shape.", 1))
	out2, lines2 := collect()
	if code := Check(root, "marked", out2); code != 1 {
		t.Fatalf("uppercase TODO must block: exit %d\n%s", code, strings.Join(*lines2, "\n"))
	}
}
