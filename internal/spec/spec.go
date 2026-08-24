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
	"procoder/internal/analysis"
	"procoder/internal/answers"
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

<!-- What this change WILL do. Bullet points, each one buildable, each
     labelled "- [S-1] ...". Every id must be cited by at least one
     acceptance criterion below: backlog seed writes one story per
     criterion, so scope no criterion cites becomes no story, no sprint,
     and ships unbuilt. The check is on the ids, not the prose — guessing
     whether a criterion covers a bullet would fail open. -->

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
     reviewer can verify, not a feeling. These seed the todo tasks.
     Each cites the scope it tests — "- [ ] [S-1] ..." — and one
     criterion may cite several ids where it genuinely covers them. -->

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
		if code := checkOne(root, f, out); code > worst {
			worst = code
		}
	}
	return worst
}

// OpenQuestions is the questions a spec still carries: the lines of real
// content in its Open questions section, with a question wrapped over several
// lines read as the one question it is. It lives here because this package
// decides what a spec is; `ask` offers these to a human and `checkOne` judges
// them, and one reading keeps the two from disagreeing.
func OpenQuestions(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	body := textutil.StripComments(textutil.Section(string(raw), "Open questions"))
	var out []string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if continuesPrevious(t) && len(out) > 0 {
			out[len(out)-1] += " " + t
			continue
		}
		// The bullet is punctuation, not part of the question: keeping it
		// would put "- " inside the key, so changing a dash to an asterisk
		// would silently discard the answer. A leading "[O-3] " label is the
		// same kind of noise — specs number their open questions, and
		// deleting question two renumbers every question below it, which
		// would orphan every answer already given to them.
		out = append(out, strings.TrimSpace(questionLabel.ReplaceAllString(
			strings.TrimPrefix(strings.TrimPrefix(t, "- "), "* "), "")))
	}
	return out
}

// questionLabel matches the "[O-3] " a spec puts in front of an open
// question. Only that shape: letters, a dash, digits. A question that opens
// with real bracketed prose — "[Windows] does the launcher…" — keeps it,
// because there the bracket is the question.
var questionLabel = regexp.MustCompile(`^\[[A-Za-z]+-\d+\]\s*`)

// continuesPrevious reports whether a line is the rest of the question above
// it rather than a new one. Counting lines instead made a four-question
// section ask six times, each fragment carrying its own key, so answering the
// whole thing still left two halves of a sentence outstanding.
func continuesPrevious(line string) bool {
	if strings.HasPrefix(line, "OPEN:") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return false
	}
	// A line that ends in a question mark is a question, bullet or not.
	// Without this, a section written as plain lines collapsed into one
	// question with one key, and answering it retired every question in it.
	if strings.HasSuffix(line, "?") {
		return false
	}
	if head, _, ok := strings.Cut(line, "."); ok && head != "" && strings.Trim(head, "0123456789") == "" {
		return false // "1. …" numbered question
	}
	return true
}

// openQuestions splits a spec's remaining questions into the ones nobody has
// answered and the count that a human already decided through `procoder ask`.
// The reading of the section itself is OpenQuestions above, which the ask
// package calls too, so the collector that offers a question and the checker
// that judges it can never disagree about what is still being asked.
func openQuestions(root, path string) (unanswered []string, answered int) {
	questions := OpenQuestions(path)
	if len(questions) == 0 {
		return nil, 0
	}
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	recorded, err := answers.Load(root)
	if err != nil {
		// Unknown is not answered: an unreadable answers file must not
		// quietly bless a spec nobody has decided.
		return questions, 0
	}
	for _, q := range questions {
		if _, ok := recorded[answers.Key("spec", name, q)]; ok {
			answered++
			continue
		}
		unanswered = append(unanswered, q)
	}
	return unanswered, answered
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

func checkOne(root, path string, out func(string)) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		out(filepath.Base(path) + ": unreadable — " + err.Error())
		return 2
	}
	text := string(raw)
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	var gaps, notes []string
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
	// The section itself, not just the marker. Two specs in this repository
	// carried seven undecided questions between them written as `- [O-1] …`
	// and were reported COMPLETE, because the rule only recognised the
	// `OPEN:` prefix — and `backlog seed` refuses anything that is not
	// COMPLETE, so it would have seeded stories from a design nobody had
	// finished. A question is unresolved because it is still in the
	// section, whatever it is called.
	if open, answered := openQuestions(root, path); len(open) > 0 {
		gaps = append(gaps, fmt.Sprintf("Open questions still has %d unanswered question(s) — put them to the user (`procoder ask`) or rewrite each as a decision",
			len(open)))
	} else if answered > 0 {
		// D-5: a question a human has decided is decided, even while the
		// prose still lists it. The verdict moves; the silence does not —
		// a reader who is told a section full of questions is finished
		// would be misled, so the note says where the decisions live.
		notes = append(notes, fmt.Sprintf("  note: %d question(s) here are answered in %s — rewriting them as decisions is tidier, and no longer required to pass",
			answered, filepath.ToSlash(filepath.Join(answers.Dir, answers.File))))
	}
	// Every promise needs a test, or the story it becomes never exists.
	// Declared rather than inferred: a bullet wrongly judged covered
	// would be the same silence this check exists to break.
	if uncovered, declared := ScopeCoverage(text); !declared {
		if n := ScopeBullets(text); n > 0 {
			gaps = append(gaps, fmt.Sprintf("scope coverage NOT checked — the %d In scope bullet(s) carry no [S-n] ids, so nothing can tell which are tested; label each and cite the id from its criteria", n))
		}
	} else if len(uncovered) > 0 {
		gaps = append(gaps, fmt.Sprintf("scope %s promised and untested — `backlog seed` writes one story per criterion, so scope no criterion cites becomes no story, no sprint, and ships unbuilt",
			strings.Join(uncovered, ", ")))
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
	// The analysis a spec came from, when there is one. A reader asking
	// why a spec says what it says can follow the link back; a spec whose
	// author already knew what to build owes nobody such a document, so
	// nothing is said when none exists.
	if src := analysis.SpecSource(root, name); src != "" {
		notes = append(notes, "  note: this spec came from "+src)
	}
	if len(gaps) == 0 {
		out("spec " + name + ": COMPLETE — sections answered, no open questions, criteria testable")
		// The template ships `Status: draft` and nothing ever advances it, so
		// finished specs read as drafts forever. Saying it here — where the
		// verdict is already being printed — costs nothing and keeps the
		// header honest; it is a note, not a gap, so the verdict stands.
		for _, n := range notes {
			out(n)
		}
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
	// The louder half of the same rot: a header claiming complete over a spec
	// the checker is refusing sends a reader to build from an unfinished
	// design, where a stale `draft` only understates a finished one.
	if statusOf(text) == "complete" {
		out("  - the Status line says complete, and the gaps above say otherwise")
	}
	return 1
}

// Files is every spec in the repository, by path. Exported so the ask
// domain reads the same set the checker judges — two listings would
// eventually disagree about what a spec is.
func Files(root string) []string { return specFiles(root) }

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
