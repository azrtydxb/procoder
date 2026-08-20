// Package spec is the quality controller for spec-first design: specs live
// as Markdown files under .procoder/specs/, written by the agent through the
// gap-closing interview in /procoder:spec. The binary verifies completeness —
// every required section present with substance, no unresolved OPEN: markers,
// acceptance criteria in testable checkbox form — and blocks until the spec
// is actually complete. The agent interviews and writes; the binary judges
// (P-CONTROL).
package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"procoder/internal/textutil"
)

// Dir is where specs live, under D-HOME.
const Dir = ".procoder/specs"

// Sections every spec must carry, in the order the interview asks them.
// A missing section is a gap nobody asked about; an empty one is a gap
// somebody skipped.
var Sections = []string{
	"Problem",
	"Users",
	"In scope",
	"Out of scope",
	"Constraints",
	"Interfaces",
	"Data",
	"Edge cases",
	"Failure modes",
	"Acceptance criteria",
	"Open questions",
}

// Template is the shape of a spec file; `procoder spec template` prints it
// and the agent fills it through the interview.
const Template = `# %s

Status: draft

## Problem

<!-- What hurts today, for whom, and why now. One paragraph of substance. -->

## Users

<!-- Who touches this, and what each of them needs from it. -->

## In scope

<!-- What this change WILL do. Bullet points, each one buildable. -->

## Out of scope

<!-- What this change will NOT do — written down so nobody assumes it. -->

## Constraints

<!-- Hard limits: performance, compatibility, security, platform, budget. -->

## Interfaces

<!-- Commands, APIs, file formats, UI surfaces this exposes or changes. -->

## Data

<!-- What is stored, where, in what shape, and who owns it. -->

## Edge cases

<!-- The inputs and states that break naive implementations. -->

## Failure modes

<!-- What happens when each dependency is missing, slow, or wrong. -->

## Acceptance criteria

<!-- Each criterion is a testable checkbox: an observable behaviour a
     reviewer can verify, not a feeling. These seed the todo tasks. -->

- [ ] ...

## Open questions

<!-- Every unresolved decision as an OPEN: line. The spec is not ready
     while any remain — resolve each with the user, then rewrite it as a
     decision in the section it belongs to. -->

- OPEN: ...
`

var (
	openRe     = regexp.MustCompile(`(?m)^\s*(?:- )?OPEN:`)
	checkboxRe = regexp.MustCompile(`(?m)^\s*- \[[ xX]\]\s*(.+)$`)
	vagueRe    = regexp.MustCompile(`(?i)\b(works well|user.?friendly|fast enough|good|nice|clean|properly|correctly|as expected|etc\.?)\s*$`)
	statusRe   = regexp.MustCompile(`(?m)^Status:\s*(.+)$`)
)

// List names every spec in the repo.
func List(root string, out func(string)) int {
	names := specFiles(root)
	if len(names) == 0 {
		out("no specs yet — /procoder:spec starts the interview, `procoder spec template <name>` prints the shape")
		return 0
	}
	for _, n := range names {
		out("  " + strings.TrimSuffix(filepath.Base(n), ".md"))
	}
	return 0
}

// PrintTemplate hands the agent the spec shape to write (P-CONTROL: the
// binary creates no files).
func PrintTemplate(name string, out func(string)) int {
	if name == "" || name != filepath.Base(name) || strings.Contains(name, "..") {
		out("a spec needs a plain file name")
		return 2
	}
	out("== write this to " + filepath.Join(Dir, name+".md") + " and fill it through the interview:")
	out(fmt.Sprintf(Template, name))
	return 0
}

// Check is the quality controller: it verifies one spec (or all) and blocks
// while gaps remain, naming each one.
func Check(root, name string, out func(string)) int {
	var files []string
	if name == "" || name == "all" {
		files = specFiles(root)
		if len(files) == 0 {
			out("no specs to check — nothing under " + Dir)
			return 0
		}
	} else {
		if name != filepath.Base(name) || strings.Contains(name, "..") {
			out(fmt.Sprintf("invalid spec name %q — names are plain file names", name))
			return 2
		}
		f := filepath.Join(root, Dir, name+".md")
		if _, err := os.Stat(f); err != nil {
			out("no spec " + name + " — `procoder spec list` shows what exists")
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

// statusOf reads the spec's `Status:` header, lowercased, or "" when the
// file carries none.
func statusOf(text string) string {
	m := statusRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(m[1]))
}

func checkOne(path string, out func(string)) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		out(filepath.Base(path) + ": unreadable — " + err.Error())
		return 2
	}
	text := string(raw)
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	var gaps []string
	for _, s := range Sections {
		body := textutil.Section(text, s)
		if body == "" && !strings.Contains(text, "## "+s) {
			gaps = append(gaps, "section missing: "+s)
			continue
		}
		if strings.TrimSpace(textutil.StripComments(body)) == "" && s != "Open questions" {
			gaps = append(gaps, "section empty: "+s+" — a heading is not an answer")
		}
	}
	if n := len(openRe.FindAllString(text, -1)); n > 0 {
		gaps = append(gaps, fmt.Sprintf("%d unresolved OPEN: question(s) — resolve each with the user and rewrite it as a decision", n))
	}
	criteria := textutil.Section(text, "Acceptance criteria")
	boxes := checkboxRe.FindAllStringSubmatch(criteria, -1)
	if strings.Contains(text, "## Acceptance criteria") && len(boxes) == 0 {
		gaps = append(gaps, "Acceptance criteria carry no checkboxes — each criterion must be a testable `- [ ]` line")
	}
	for _, m := range boxes {
		c := strings.TrimSpace(m[1])
		if c == "..." {
			gaps = append(gaps, "Acceptance criteria still contain the placeholder")
			continue
		}
		if vagueRe.MatchString(c) {
			gaps = append(gaps, "untestable criterion: "+trim(c)+" — say what a reviewer would observe, not how it should feel")
		}
	}
	if len(gaps) == 0 {
		out("spec " + name + ": COMPLETE — sections answered, no open questions, criteria testable")
		// The template ships `Status: draft` and nothing ever advances it, so
		// finished specs read as drafts forever. Saying it here — where the
		// verdict is already being printed — costs nothing and keeps the
		// header honest; it is a note, not a gap, so the verdict stands.
		if statusOf(text) == "draft" {
			out("  note: the Status line still says draft — advance it to `Status: complete`")
		}
		out("  next: seed todos from the acceptance criteria (`procoder todo add`), one task per criterion group")
		return 0
	}
	out("spec " + name + ": NOT ready — the quality controller found:")
	for _, g := range gaps {
		out("  - " + g)
	}
	return 1
}

func specFiles(root string) []string {
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

func trim(s string) string {
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}
