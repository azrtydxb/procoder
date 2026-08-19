// Package todo is the quality-gated task system: tasks live as Markdown
// files under .procoder/todo/, each carrying a real description, explicit
// acceptance criteria, and an evidence section. The binary is the quality
// controller — a task closes ONLY when every criterion is checked, the
// evidence says what proved it, and the gate is clean. The agent edits the
// files; the binary verifies and refuses (P-CONTROL).
package todo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"procoder/internal/textutil"
)

// Dir is where tasks live, under D-HOME.
const Dir = ".procoder/todo"

// Template is the shape of a task file; `procoder todo add` prints it
// filled with the title, and the agent writes the substance.
const Template = `# %s

Status: open
Created: %s

## Description

<!-- What this task is, why it exists, and what "done" looks like in the
     reader's terms. A title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] ...

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the task open. -->
`

var (
	statusRe    = regexp.MustCompile(`(?m)^Status:\s*(\S+)`)
	uncheckedRe = regexp.MustCompile(`(?m)^\s*- \[ \]`)
	checkedRe   = regexp.MustCompile(`(?m)^\s*- \[[xX]\]`)
	placeholder = regexp.MustCompile(`(?m)^\s*- \[.\]\s*\.\.\.`)
)

// File resolves a task id to its path, refusing ids that would escape the
// todo directory (separators, dot-dot, hidden traversal).
func File(root, id string) (string, error) {
	if id == "" || id != filepath.Base(id) || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid task id %q — ids are plain file names", id)
	}
	return filepath.Join(root, Dir, id+".md"), nil
}

// Task is one parsed task file.
type Task struct {
	ID     string
	Title  string
	Status string
	Path   string
}

// List returns every task, open first, sorted by id.
func List(root string) ([]Task, error) {
	dir := filepath.Join(root, Dir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("todo directory unreadable: %v", err)
	}
	var tasks []Task
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		t := Task{ID: strings.TrimSuffix(e.Name(), ".md"), Path: path, Status: "open"}
		raw, err := os.ReadFile(path)
		if err != nil {
			// an unreadable task is a finding, not something to hide
			t.Status = "unreadable"
			t.Title = err.Error()
			tasks = append(tasks, t)
			continue
		}
		if m := statusRe.FindSubmatch(raw); m != nil {
			t.Status = string(m[1])
		}
		if i := strings.Index(string(raw), "# "); i >= 0 {
			line := string(raw)[i+2:]
			if j := strings.IndexByte(line, '\n'); j > 0 {
				t.Title = strings.TrimSpace(line[:j])
			}
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Status != tasks[j].Status {
			return tasks[i].Status == "open"
		}
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}

// Add prints the task file content and the path to write it — the binary
// creates no files (P-CONTROL); the agent reviews and writes.
func Add(root, title string, out func(string)) int {
	slug := textutil.Slug(title)
	if slug == "" {
		out("a task needs a title")
		return 2
	}
	now := time.Now().UTC()
	id := now.Format("20060102") + "-" + slug
	out("== write this to " + filepath.Join(Dir, id+".md") + ":")
	out(fmt.Sprintf(Template, title, now.Format("2006-01-02")))
	out("then fill Description and real Acceptance criteria before starting work.")
	return 0
}

// Close is the quality controller: it verifies and either closes the task
// (rewrites Status) or refuses with exactly what is missing. gateClean runs
// `procoder check` on demand — the gate result belongs in the verdict
// because "done" includes not having broken anything.
func Close(root, id string, gateClean func() bool, out func(string)) int {
	return CloseWith(root, id, gateClean, nil, out)
}

// CloseWith is Close plus the test-suite verdict: under `[test] policy =
// "block"` the caller passes testrun.Suite and a red (or unverifiable)
// suite keeps the task open. suite nil means the policy is off.
func CloseWith(root, id string, gateClean func() bool, suite func() (bool, string), out func(string)) int {
	path, err := File(root, id)
	if err != nil {
		out(err.Error())
		return 2
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		out("no task " + id + " — `procoder todo list` shows what exists")
		return 2
	}
	text := string(raw)
	var missing []string
	if m := statusRe.FindStringSubmatch(text); m != nil && m[1] != "open" {
		out("task " + id + " is already " + m[1])
		return 0
	}
	desc := textutil.Section(text, "Description")
	if strings.TrimSpace(textutil.StripComments(desc)) == "" {
		missing = append(missing, "Description is empty — a title is not a description")
	}
	criteria := textutil.Section(text, "Acceptance criteria")
	if placeholder.MatchString(criteria) {
		missing = append(missing, "Acceptance criteria still contain the placeholder — write real, testable criteria")
	}
	if !checkedRe.MatchString(criteria) {
		missing = append(missing, "no acceptance criterion is checked")
	}
	if n := len(uncheckedRe.FindAllString(criteria, -1)); n > 0 {
		missing = append(missing, fmt.Sprintf("%d acceptance criterion(s) unchecked", n))
	}
	evidence := textutil.Section(text, "Evidence")
	if strings.TrimSpace(textutil.StripComments(evidence)) == "" {
		missing = append(missing, "Evidence is empty — record the commands run and what their output proved")
	}
	if !gateClean() {
		missing = append(missing, "the gate is not clean — `procoder check` must pass before a task closes")
	}
	if suite != nil {
		if ok, summary := suite(); !ok {
			missing = append(missing, "the test suite is not green — "+summary)
		}
	}
	if len(missing) > 0 {
		out("task " + id + " stays OPEN — the quality controller found:")
		for _, m := range missing {
			out("  - " + m)
		}
		return 1
	}
	closed := statusRe.ReplaceAllString(text, "Status: closed "+time.Now().UTC().Format("2006-01-02"))
	if err := os.WriteFile(path, []byte(closed), 0o644); err != nil {
		out("cannot update the task file: " + err.Error())
		return 2
	}
	out("task " + id + " closed — criteria checked, evidence recorded, gate clean")
	return 0
}
