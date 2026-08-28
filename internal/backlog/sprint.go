package backlog

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"procoder/internal/config"
	"procoder/internal/textutil"
)

// sprintLineRe matches the whole Sprint header line of a story file — the
// one line pull and carry are allowed to rewrite (whole-file write, one
// targeted replacement; nothing else in the story is the binary's to touch).
var sprintLineRe = regexp.MustCompile(`(?m)^Sprint:.*$`)

const sprintTemplate = `# %s

Status: active
Created: %s

## Goal

<!-- What this sprint commits to deliver, in the reader's terms — the
     outcome the stories add up to, not a list of them. -->
`

// activeSprint finds the one sprint with Status active. An unreadable
// sprint file is an error, not a shrug — it might be the active one, and
// guessing would let a second sprint open behind it.
func activeSprint(root string) (Item, bool, error) {
	items, err := LoadAll(root)
	if err != nil {
		return Item{}, false, err
	}
	for _, it := range items {
		if it.Kind != KindSprint {
			continue
		}
		if it.Status == "unreadable" {
			return Item{}, false, fmt.Errorf("sprint file %s is unreadable — an unreadable sprint might be active; fix it before opening or closing anything", filepath.ToSlash(filepath.Join(Dir, KindSprint, it.ID+".md")))
		}
		if it.Status == "active" {
			return it, true, nil
		}
	}
	return Item{}, false, nil
}

// SprintOpen prints the next sprint file for the agent to write
// (P-CONTROL). One sprint at a time is the WIP limit that makes a sprint
// mean something — a second open is refused, never queued.
func SprintOpen(root, goal string, out func(string)) int {
	if strings.TrimSpace(goal) == "" || textutil.Slug(goal) == "" {
		out("a sprint needs a goal — `procoder sprint open <goal words...>`")
		return 2
	}
	active, ok, err := activeSprint(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if ok {
		out("sprint " + active.ID + " is still active — one sprint at a time; close it with `procoder sprint close` first")
		return 1
	}
	items, err := LoadAll(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	// Next sequence number: one past the highest existing sprint's, so
	// the directory reads as the project's sprint history in order. The
	// same walk finds the most recently closed sprint for the retro gate.
	seq := 0
	var last Item
	lastSeq := -1
	for _, it := range items {
		if it.Kind != KindSprint {
			continue
		}
		n := sprintSeq(it.ID)
		if n > seq {
			seq = n
		}
		if strings.HasPrefix(it.Status, "closed") && (n > lastSeq || (n == lastSeq && it.ID > last.ID)) {
			last, lastSeq = it, n
		}
	}
	if lastSeq >= 0 && !config.Load(root).SprintRetroOff {
		if code := retroGate(root, last, out); code != 0 {
			return code
		}
	}
	id := fmt.Sprintf("%03d-%s", seq+1, textutil.Slug(goal))
	rel := filepath.ToSlash(filepath.Join(Dir, KindSprint, id+".md"))
	out("== write this to " + rel + ":")
	out(fmt.Sprintf(sprintTemplate, goal, time.Now().UTC().Format("2006-01-02")))
	out("then pull the stories this goal needs with `procoder sprint pull <story-id>`.")
	return 0
}

// sprintSeq reads the leading digits of a sprint id as its sequence
// number; an id with no leading digits has no place in the sequence (-1).
func sprintSeq(id string) int {
	digits := id
	if i := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }); i >= 0 {
		digits = digits[:i]
	}
	if n, err := strconv.Atoi(digits); err == nil {
		return n
	}
	return -1
}

// retroGate is the price of the next sprint: the last closed sprint's
// Retro section must hold real content — guidance comments stripped —
// before another sprint opens. A closed sprint with no Retro section at
// all (one that predates the scaffold) counts as empty, and an unreadable
// one refuses too: an unreadable retro is unknown, and unknown is never
// done. `[sprint] retro = "off"` in config.toml opts a repo out.
func retroGate(root string, last Item, out func(string)) int {
	rel := filepath.ToSlash(filepath.Join(Dir, KindSprint, last.ID+".md"))
	raw, err := readUnder(root, last.Path)
	if err != nil {
		out("cannot read " + rel + " — an unreadable retro is unknown, and unknown is never done; fix the file before opening a sprint")
		return 1
	}
	if strings.TrimSpace(textutil.StripComments(textutil.Section(string(raw), "Retro"))) == "" {
		out("the retro is the price of the next sprint — write what sprint " + last.ID + " taught you into the ## Retro section of " + rel + " (or set `[sprint] retro = \"off\"` in .procoder/config.toml)")
		return 1
	}
	return 0
}

// SprintPull commits stories to the active sprint — a verified transition,
// so the binary rewrites the Sprint line itself. Each id is judged on its
// own: one bad argument must not block the rest of the batch.
func SprintPull(root string, ids []string, out func(string)) int {
	active, ok, err := activeSprint(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if !ok {
		out("no active sprint — `procoder sprint open <goal>` starts one")
		return 1
	}
	refused := false
	for _, id := range ids {
		path, err := ItemFile(root, KindStory, id)
		if err != nil {
			out("refused " + id + ": " + err.Error())
			refused = true
			continue
		}
		raw, err := readUnder(root, path)
		if err != nil {
			out("refused " + id + ": no such story — `procoder backlog list` shows what exists")
			refused = true
			continue
		}
		text := string(raw)
		it := Item{Status: "open"}
		if m := statusRe.FindStringSubmatch(text); m != nil {
			it.Status = m[1]
		}
		if it.Done() {
			out("refused " + id + ": already done — pulling finished work inflates the sprint")
			refused = true
			continue
		}
		sprint := ""
		if m := sprintRe.FindStringSubmatch(text); m != nil {
			sprint = m[1]
		}
		if sprint != "" && sprint != "-" {
			out("refused " + id + ": already committed to sprint " + sprint)
			refused = true
			continue
		}
		if !sprintLineRe.MatchString(text) {
			out("refused " + id + ": no Sprint: header line — the story file is not in the expected shape")
			refused = true
			continue
		}
		updated := sprintLineRe.ReplaceAllString(text, "Sprint: "+active.ID)
		if err := writeUnder(root, path, []byte(updated)); err != nil {
			out("cannot update the story file: " + err.Error())
			return 2
		}
		out("pulled " + id + " into sprint " + active.ID)
	}
	if refused {
		return 1
	}
	return 0
}

// SprintCarry sends an unfinished story back to the backlog with the why
// written down. Lean's rule: unfinished work is visible, never silent —
// the Carried line is the sprint's honest trace on the story.
func SprintCarry(root, id, reason string, out func(string)) int {
	if strings.TrimSpace(reason) == "" {
		out("carrying without a reason hides the why — lean makes unfinished work visible: `procoder sprint carry <story-id> <reason words...>`")
		return 2
	}
	active, ok, err := activeSprint(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if !ok {
		out("no active sprint — `procoder sprint open <goal>` starts one")
		return 1
	}
	path, err := ItemFile(root, KindStory, id)
	if err != nil {
		out(err.Error())
		return 2
	}
	raw, err := readUnder(root, path)
	if err != nil {
		out("no story " + id + " — `procoder backlog list` shows what exists")
		return 1
	}
	text := string(raw)
	sprint := ""
	if m := sprintRe.FindStringSubmatch(text); m != nil {
		sprint = m[1]
	}
	if sprint != active.ID {
		out("story " + id + " is not in sprint " + active.ID + " — only committed stories carry back")
		return 1
	}
	updated := sprintLineRe.ReplaceAllString(text, "Sprint: -\nCarried: "+active.ID+" — "+reason)
	if err := writeUnder(root, path, []byte(updated)); err != nil {
		out("cannot update the story file: " + err.Error())
		return 2
	}
	out("story " + id + " carried back to the backlog — Carried: " + active.ID + " recorded")
	return 0
}

// SprintStatus is the stand-up view: what the sprint committed to, where
// each story stands, and what already fell out of scope.
func SprintStatus(root string, out func(string)) int {
	active, ok, err := activeSprint(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if !ok {
		// Exit 0, not 1, and only here. ADR 0003 reserves 1 for findings
		// or a refusal: pulling, carrying and closing with no sprint open
		// are refusals and keep it, but ASKING which sprint is running is
		// a question, and "none" is its answer — the way `todo list`
		// answers "no tasks" with 0. The two disagreed until the fixture
		// campaign put them side by side, so a script asking whether a
		// sprint was running read a normal state as a failure.
		out("no active sprint — `procoder sprint open <goal>` starts one")
		return 0
	}
	out("sprint " + active.ID + "  " + active.Title)
	committed, carried := sprintStories(root, active.ID)
	done := 0
	for _, s := range committed {
		mark := " "
		if s.Done() {
			mark = "x"
			done++
		}
		out("  [" + mark + "] " + s.ID + "  " + s.Title)
	}
	line := fmt.Sprintf("%d of %d stories done", done, len(committed))
	if len(carried) > 0 {
		line += fmt.Sprintf(", %d carried back", len(carried))
	}
	out(line)
	return 0
}

// SprintClose is the controller: every committed story must be done or
// explicitly carried before the sprint closes, and the close writes the
// honest tally — committed, done, carried — into the sprint file.
func SprintClose(root string, out func(string)) int {
	active, ok, err := activeSprint(root)
	if err != nil {
		out(err.Error())
		return 1
	}
	if !ok {
		out("no active sprint — `procoder sprint open <goal>` starts one")
		return 1
	}
	committed, carried := sprintStories(root, active.ID)
	var open, done []string
	for _, s := range committed {
		if s.Done() {
			done = append(done, s.ID)
		} else {
			open = append(open, s.ID)
		}
	}
	if len(open) > 0 {
		out("sprint " + active.ID + " stays ACTIVE — the quality controller found:")
		for _, id := range open {
			out("  - " + id + " is neither done nor carried — finish it, or `procoder sprint carry " + id + " <reason>`")
		}
		return 1
	}
	raw, err := readUnder(root, active.Path)
	if err != nil {
		out("cannot read the sprint file: " + err.Error())
		return 2
	}
	text := statusRe.ReplaceAllString(string(raw), "Status: closed "+time.Now().UTC().Format("2006-01-02"))
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += "\n## Result\n\n" +
		fmt.Sprintf("committed: %d\n", len(done)+len(carried)) +
		"done: " + countWithIDs(done) + "\n" +
		"carried: " + countWithIDs(carried) + "\n"
	// The retro scaffold, only where the template did not already supply
	// one. Appending it unconditionally gave every sprint two ## Retro
	// headings — and if the person had already written their retro in the
	// first, the empty second one sat underneath it, so the next sprint's
	// check could read either.
	if !retroRe.MatchString(text) {
		text += "\n## Retro\n\n" +
			"<!-- What slowed us down this sprint. -->\n\n" +
			"<!-- What we change next sprint because of it. -->\n\n" +
			"<!-- One adaptation from this sprint worth keeping. -->\n"
	}
	if err := writeUnder(root, active.Path, []byte(text)); err != nil {
		out("cannot update the sprint file: " + err.Error())
		return 2
	}
	out(fmt.Sprintf("sprint %s closed — %d committed, %d done, %d carried", active.ID, len(done)+len(carried), len(done), len(carried)))
	return 0
}

// sprintStories splits the stories a sprint touched: those still committed
// (Sprint field is the sprint id) and those carried back (a Carried line
// in the story file names this sprint).
func sprintStories(root, sprintID string) (committed []Item, carried []string) {
	items, err := LoadAll(root)
	if err != nil {
		return nil, nil
	}
	carriedRe := regexp.MustCompile(`(?m)^Carried:\s*` + regexp.QuoteMeta(sprintID) + `\b`)
	for _, it := range items {
		if it.Kind != KindStory {
			continue
		}
		if it.Sprint == sprintID {
			committed = append(committed, it)
			continue
		}
		raw, err := readUnder(root, it.Path)
		if err == nil && carriedRe.Match(raw) {
			carried = append(carried, it.ID)
		}
	}
	sort.Slice(committed, func(a, b int) bool { return committed[a].ID < committed[b].ID })
	sort.Strings(carried)
	return committed, carried
}

// countWithIDs formats "N (id, id)" — or a bare "0" when there is nothing
// to name, the zero summary an empty sprint closes with.
func countWithIDs(ids []string) string {
	if len(ids) == 0 {
		return "0"
	}
	return fmt.Sprintf("%d (%s)", len(ids), strings.Join(ids, ", "))
}
