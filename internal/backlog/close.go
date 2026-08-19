// The close controllers: a story closes only when it carries todo-task
// rigor and the gate is clean; an epic closes only when every story on it
// is done; a milestone closes only when every epic under it is done. Each
// refusal lists ALL gaps found — not the first — and names the next
// action. Closing is the ONLY thing here that writes: a whole-file rewrite
// of the Status line, exactly the todo.Close precedent (P-CONTROL).
package backlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CloseStory is the quality controller for the execution unit: it verifies
// and either closes the story (rewrites Status) or refuses with exactly
// what is missing. gateClean runs `procoder check` on demand — the gate
// result belongs in the verdict because "done" includes not having broken
// anything. Mirrors todo.Close in full rigor; the two domains stay
// independent.
func CloseStory(root, id string, gateClean func() bool, out func(string)) int {
	return CloseStoryWith(root, id, gateClean, nil, out)
}

// CloseStories closes several stories against ONE verification. The gate
// and the suite judge the tree, not a story, so asking them once per story
// re-runs the same answer N times — a sprint's worth of closes cost
// thirteen minutes of identical work. Each story is still judged on its
// own criteria and evidence, and one incomplete story is refused by name
// without costing the others their close.
func CloseStories(root string, ids []string, gateClean func() bool, suite func() (bool, string), out func(string)) int {
	gateOnce := once(gateClean)
	var suiteOnce func() (bool, string)
	if suite != nil {
		suiteOnce = onceSuite(suite)
	}
	worst := 0
	for _, id := range ids {
		if code := CloseStoryWith(root, id, gateOnce, suiteOnce, out); code > worst {
			worst = code
		}
	}
	return worst
}

// once memoizes the gate verdict for the life of one batch: the tree does
// not change while the batch runs, so a second run could only waste time.
func once(f func() bool) func() bool {
	var done bool
	var result bool
	return func() bool {
		if !done {
			result, done = f(), true
		}
		return result
	}
}

// onceSuite is once for the suite verdict, which carries a summary too.
func onceSuite(f func() (bool, string)) func() (bool, string) {
	var done, ok bool
	var summary string
	return func() (bool, string) {
		if !done {
			ok, summary = f()
			done = true
		}
		return ok, summary
	}
}

// CloseStoryWith is CloseStory plus the test-suite verdict: under
// `[test] policy = "block"` the caller passes testrun.Suite and a red (or
// unverifiable) suite keeps the story open. suite nil means the policy is
// off.
func CloseStoryWith(root, id string, gateClean func() bool, suite func() (bool, string), out func(string)) int {
	path, err := ItemFile(root, KindStory, id)
	if err != nil {
		out(err.Error())
		return 2
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		out("no story " + id + " — `procoder backlog list` shows what exists")
		return 2
	}
	text := string(raw)
	if m := statusRe.FindStringSubmatch(text); m != nil && (Item{Status: m[1]}).Done() {
		out("story " + id + " is already " + m[1])
		return 0
	}
	var missing []string
	desc := section(text, "Description")
	if strings.TrimSpace(stripComments(desc)) == "" {
		missing = append(missing, "Description is empty — a title is not a description")
	}
	criteria := section(text, "Acceptance criteria")
	if placeholder.MatchString(criteria) {
		missing = append(missing, "Acceptance criteria still contain the placeholder — write real, testable criteria")
	}
	if !checkedRe.MatchString(criteria) {
		missing = append(missing, "no acceptance criterion is checked")
	}
	if n := len(uncheckedRe.FindAllString(criteria, -1)); n > 0 {
		missing = append(missing, fmt.Sprintf("%d acceptance criterion(s) unchecked", n))
	}
	if m := typeRe.FindStringSubmatch(text); m != nil && m[1] == "bug" {
		// A defect without a severity was never triaged — the header is
		// part of a bug's rigor, exactly like evidence.
		sev := ""
		if m := severityRe.FindStringSubmatch(text); m != nil {
			sev = m[1]
		}
		if !validSeverity(sev) {
			missing = append(missing, "a bug closes with a severity — add Severity: s1..s4")
		}
	}
	evidence := section(text, "Evidence")
	if strings.TrimSpace(stripComments(evidence)) == "" {
		missing = append(missing, "Evidence is empty — record the commands run and what their output proved")
	}
	if !gateClean() {
		missing = append(missing, "the gate is not clean — `procoder check` must pass before a story closes")
	}
	if suite != nil {
		if ok, summary := suite(); !ok {
			missing = append(missing, "the test suite is not green — "+summary)
		}
	}
	if len(missing) > 0 {
		out("story " + id + " stays OPEN — the quality controller found:")
		for _, m := range missing {
			out("  - " + m)
		}
		return 1
	}
	if !markDone(path, text, out) {
		return 2
	}
	out("story " + id + " closed — criteria checked, evidence recorded, gate clean")
	return 0
}

// CloseEpic refuses while any story referencing this epic is not done. An
// unreadable story blocks too — its epic is unknowable, and unknown ≠ done.
// Spec drift (the source spec changed or vanished since seeding) is a
// warning on both refusal and success, never a block: the stories are the
// contract, the fingerprint is traceability.
func CloseEpic(root, id string, out func(string)) int {
	items, err := LoadAll(root)
	if err != nil {
		out(err.Error())
		return 2
	}
	epic, ok := findItem(items, KindEpic, id)
	if !ok {
		out("no epic " + id + " — `procoder backlog list` shows what exists")
		return 2
	}
	if epic.Done() {
		out("epic " + id + " is already " + epic.Status)
		return 0
	}
	warns := driftWarnings(root, epic)
	var open []string
	for _, it := range items {
		if it.Kind != KindStory || it.Done() {
			continue
		}
		switch {
		case it.Status == "unreadable":
			// An unreadable story might belong to this epic — its Epic
			// field could not be parsed, so it blocks conservatively.
			open = append(open, it.ID+" is unreadable — unknown ≠ done; fix or remove the file")
		case it.Epic == id:
			open = append(open, it.ID+" is "+it.Status)
		}
	}
	if len(open) > 0 {
		out("epic " + id + " stays OPEN — these stories are not done:")
		for _, l := range open {
			out("  - " + l)
		}
		emit(out, warns)
		return 1
	}
	raw, err := os.ReadFile(epic.Path)
	if err != nil {
		out("cannot read the epic file: " + err.Error())
		return 2
	}
	if !markDone(epic.Path, string(raw), out) {
		return 2
	}
	out("epic " + id + " closed — every story done")
	emit(out, warns)
	return 0
}

// CloseMilestone refuses while any epic referencing this milestone is not
// done — the same shape as CloseEpic one level up, unreadable epics
// included (unknown ≠ done).
func CloseMilestone(root, id string, out func(string)) int {
	items, err := LoadAll(root)
	if err != nil {
		out(err.Error())
		return 2
	}
	ms, ok := findItem(items, KindMilestone, id)
	if !ok {
		out("no milestone " + id + " — `procoder backlog list` shows what exists")
		return 2
	}
	if ms.Done() {
		out("milestone " + id + " is already " + ms.Status)
		return 0
	}
	var open []string
	for _, it := range items {
		if it.Kind != KindEpic || it.Done() {
			continue
		}
		switch {
		case it.Status == "unreadable":
			open = append(open, it.ID+" is unreadable — unknown ≠ done; fix or remove the file")
		case it.Milestone == id:
			open = append(open, it.ID+" is "+it.Status)
		}
	}
	if len(open) > 0 {
		out("milestone " + id + " stays OPEN — these epics are not done:")
		for _, l := range open {
			out("  - " + l)
		}
		return 1
	}
	raw, err := os.ReadFile(ms.Path)
	if err != nil {
		out("cannot read the milestone file: " + err.Error())
		return 2
	}
	if !markDone(ms.Path, string(raw), out) {
		return 2
	}
	out("milestone " + id + " closed — every epic done")
	return 0
}

// findItem returns the one item of a kind with this id.
func findItem(items []Item, kind, id string) (Item, bool) {
	for _, it := range items {
		if it.Kind == kind && it.ID == id {
			return it, true
		}
	}
	return Item{}, false
}

// driftWarnings compares the epic's recorded spec fingerprint against the
// spec file as it stands now. Missing or changed specs are traceability
// findings, not blockers — say so, then let the close proceed.
func driftWarnings(root string, epic Item) []string {
	if epic.SpecName == "" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(root, ".procoder", "specs", epic.SpecName+".md"))
	if err != nil {
		return []string{"⚠ spec " + epic.SpecName + " is missing — traceability lost, not blocking"}
	}
	if fingerprint(b) != epic.SpecPrint {
		return []string{"⚠ spec " + epic.SpecName + " changed since seeding — review the stories before trusting them (not blocking)"}
	}
	return nil
}

func emit(out func(string), lines []string) {
	for _, l := range lines {
		out(l)
	}
}

// markDone rewrites the Status line to done <date> in a whole-file write —
// no partial rewrite, same as todo. A write failure is printed verbatim.
func markDone(path, text string, out func(string)) bool {
	done := statusRe.ReplaceAllString(text, "Status: done "+time.Now().UTC().Format("2006-01-02"))
	if err := os.WriteFile(path, []byte(done), 0o644); err != nil {
		out("cannot update the file: " + err.Error())
		return false
	}
	return true
}
