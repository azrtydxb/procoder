// Package plan is the quality controller for implementation plans: the
// middle link of the spec → plan → todo chain. Plans live under
// .procoder/plans/, written by the agent from an approved spec; the binary
// verifies the plan is executable by an engineer with zero context — no
// placeholders, every task carrying its files and testable steps — and
// blocks until it is. The agent writes; the binary judges (P-CONTROL).
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"procoder/internal/textutil"
)

// Dir is where plans live, under D-HOME.
const Dir = ".procoder/plans"

// Template is the shape of a plan file; `procoder plan template` prints it
// and the agent fills it from the spec.
const Template = `# %s — implementation plan

Status: draft
Spec: <!-- path to the .procoder/specs/ file this plan implements -->

## Goal

<!-- One sentence. -->

## Architecture

<!-- Two or three sentences: the shape of the change. -->

## Constraints

<!-- Project-wide requirements every task inherits: version floors,
     naming rules, exact copy — copied verbatim from the spec. -->

## Task 1: <!-- name -->

Files: <!-- each file created or modified, one responsibility each -->
Interfaces: <!-- exact names and signatures neighbouring tasks consume or
     produce — a task's implementer sees only their own task -->

- [ ] <!-- one action per step, with the literal code or command and the
     expected result; a reviewer could reject this task alone -->
`

// placeholders that mark a plan as not actually written. "Similar to task
// N" is here because an engineer may read tasks out of order — repeat the
// code instead.
var placeholderRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bTBD\b`),
	// case-sensitive on purpose: the placeholder convention is uppercase
	// TODO, while lowercase "todo" legitimately names procoder's own task
	// domain — a plan touching internal/todo must be writable
	regexp.MustCompile(`\bTODO\b`),
	regexp.MustCompile(`(?i)implement (this |it )?later`),
	regexp.MustCompile(`(?i)add appropriate `),
	regexp.MustCompile(`(?i)handle edge cases`),
	regexp.MustCompile(`(?i)similar to task`),
	regexp.MustCompile(`(?i)write tests for the above`),
}

var (
	taskRe     = regexp.MustCompile(`(?m)^## Task (\d+)[:\s]`)
	checkboxRe = regexp.MustCompile(`(?m)^\s*- \[[ xX]\]\s*\S`)
	filesRe    = regexp.MustCompile(`(?m)^Files:\s*\S`)
	sectionRes = []string{"Goal", "Architecture", "Constraints"}
)

// List names every plan in the repo.
func List(root string, out func(string)) int {
	files := planFiles(root)
	if len(files) == 0 {
		out("no plans yet — /procoder:plan starts one from an approved spec")
		return 0
	}
	for _, f := range files {
		out("  " + strings.TrimSuffix(filepath.Base(f), ".md"))
	}
	return 0
}

// PrintTemplate hands the agent the plan shape to write (P-CONTROL: the
// binary creates no files).
func PrintTemplate(name string, out func(string)) int {
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		out("a plan needs a plain file name")
		return 2
	}
	out("== write this to " + filepath.Join(Dir, name+".md") + " and fill it from the spec:")
	out(fmt.Sprintf(Template, name))
	return 0
}

// Check is the quality controller: it verifies one plan (or all) and
// blocks while gaps remain, naming each one.
func Check(root, name string, out func(string)) int {
	var files []string
	if name == "" || name == "all" {
		files = planFiles(root)
		if len(files) == 0 {
			out("no plans to check — nothing under " + Dir)
			return 0
		}
	} else {
		if name != filepath.Base(name) || strings.Contains(name, "..") {
			out(fmt.Sprintf("invalid plan name %q — names are plain file names", name))
			return 2
		}
		f := filepath.Join(root, Dir, name+".md")
		if _, err := os.Stat(f); err != nil {
			out("no plan " + name + " — `procoder plan list` shows what exists")
			return 2
		}
		files = []string{f}
	}
	worst := 0
	for _, f := range files {
		if code := checkOne(f, out); code > worst {
			worst = code
		}
	}
	return worst
}

func checkOne(path string, out func(string)) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		out(filepath.Base(path) + ": unreadable — " + err.Error())
		return 2
	}
	text := string(raw)
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	stripped := textutil.StripComments(text)
	var gaps []string

	for _, s := range sectionRes {
		body := textutil.Section(text, s)
		if body == "" && !strings.Contains(text, "## "+s) {
			gaps = append(gaps, "section missing: "+s)
		} else if strings.TrimSpace(textutil.StripComments(body)) == "" {
			gaps = append(gaps, "section empty: "+s+" — a heading is not an answer")
		}
	}

	// placeholders anywhere in the plan body (comments excluded — the
	// template's own guidance must not flag an otherwise-complete plan)
	for _, re := range placeholderRes {
		if loc := re.FindStringIndex(stripped); loc != nil {
			gaps = append(gaps, fmt.Sprintf("placeholder %q — a plan is written, not promised", stripped[loc[0]:loc[1]]))
		}
	}

	// every task carries its files and at least one concrete step
	tasks := taskRe.FindAllStringSubmatchIndex(text, -1)
	if len(tasks) == 0 {
		gaps = append(gaps, "no `## Task N:` blocks — a plan without tasks is a wish")
	}
	for i, loc := range tasks {
		start := loc[0]
		end := len(text)
		if i+1 < len(tasks) {
			end = tasks[i+1][0]
		}
		body := textutil.StripComments(text[start:end])
		num := text[loc[2]:loc[3]]
		if !filesRe.MatchString(body) {
			gaps = append(gaps, "Task "+num+" names no Files: — the implementer cannot know what to touch")
		}
		if !checkboxRe.MatchString(body) {
			gaps = append(gaps, "Task "+num+" has no checkbox steps — steps are what makes a task executable")
		}
	}

	if len(gaps) == 0 {
		out("plan " + name + ": COMPLETE — sections answered, no placeholders, every task carries files and steps")
		out("  next: seed todos (`procoder todo add`), one task each, criteria from the plan's steps")
		return 0
	}
	out("plan " + name + ": NOT ready — the quality controller found:")
	for _, g := range gaps {
		out("  - " + g)
	}
	return 1
}

func planFiles(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, filepath.Join(root, Dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}
