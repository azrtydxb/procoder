// Package status is the state-of-play report: where this repository stands
// right now, computed fresh every time from git and procoder's own state.
// Every line is a fact the binary derived — a value that could not be read
// prints as unknown WITH the reason, never as a comfortable default. The
// report is injected at session start, so speed is part of the contract:
// the whole thing lives inside a hard budget and never runs the gate, the
// suite, or anything that touches the network.
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"procoder/internal/backlog"
	"procoder/internal/codeindex"
	"procoder/internal/gitx"
	"procoder/internal/lessons"
	"procoder/internal/testrun"
	"procoder/internal/todo"
)

// Budget is the wall for the whole report. SessionStart waits on this, so it
// is a promise, not an aspiration: whatever has not answered by then is left
// out with a note rather than delaying the session.
const Budget = 3 * time.Second

// reserve keeps the assembly of the report inside Budget: the git lookups get
// everything except this sliver, so the total stays under the wall even when
// git is the thing that ran long.
const reserve = 200 * time.Millisecond

// Header names the block, so the same words introduce `procoder status` and
// the block injected into a session.
const Header = "== state of play"

// Report returns the state-of-play lines, in reading order: git first
// (branch, head, dirty), then the project layer (sprint, stories, tasks),
// then the ledgers (lessons, index).
func Report(root string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), Budget-reserve)
	defer cancel()

	type gitResult struct {
		lines []string
		head  string
	}
	ch := make(chan gitResult, 1)
	go func() {
		lines, head := gitLines(ctx, root)
		ch <- gitResult{lines, head}
	}()

	// the file-backed lines are computed while git runs: they read small
	// local files, and doing them here keeps the total inside the budget
	project := append(sprintLines(root), taskLine(root), lessonLine(root))
	if d := deferredLine(root); d != "" {
		project = append(project, d)
	}

	var lines []string
	head, timedOut := "", false
	select {
	case r := <-ch:
		lines = append(lines, r.lines...)
		head = r.head
	case <-ctx.Done():
		timedOut = true
	}
	lines = append(lines, project...)
	lines = append(lines, indexLine(root, head, timedOut))
	if timedOut {
		lines = append(lines, "some state omitted for speed")
	}
	return lines
}

// Run prints the report. Exit 0 always — a report cannot fail; the lines say
// what could not be read.
func Run(root string, out func(string)) int {
	out(Header)
	for _, line := range Report(root) {
		out(line)
	}
	return 0
}

// --- git ---------------------------------------------------------------------

// git runs one plumbing command under the report's deadline. The error comes
// back as text because that text is what the unknown line has to say.
func git(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// gitLines computes the branch, head, and dirty-count lines plus the short
// HEAD the index line needs. Without a repository (or without git) all three
// report unknown with the reason git itself gave.
func gitLines(ctx context.Context, root string) ([]string, string) {
	gitDir, err := git(ctx, root, "rev-parse", "--git-dir")
	if err != nil {
		reason := gitReason(err)
		return []string{
			"branch: unknown — " + reason,
			"head: unknown — " + reason,
			"dirty files: unknown — " + reason,
		}, ""
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return []string{branchLine(ctx, root, gitDir), headLine(ctx, root), dirtyLine(ctx, root)}, shortHead(ctx, root)
}

// branchLine names where the work is happening and how it stands against the
// default branch. A detached HEAD and a rebase in progress are named, never
// smoothed over into a branch name that is not true.
func branchLine(ctx context.Context, root, gitDir string) string {
	current := gitx.CurrentBranch(root)
	state := rebaseState(gitDir)
	if current == "" {
		at := shortHead(ctx, root)
		if at == "" {
			return "branch: detached HEAD" + state + " — no commits yet"
		}
		return "branch: detached HEAD at " + at + state
	}
	def := gitx.DefaultBranch(root)
	if def == "" {
		return "branch: " + current + state + " — default branch unknown (no origin/HEAD, no main or master)"
	}
	if def == current {
		return "branch: " + current + state + " — this is the default branch"
	}
	counts, err := git(ctx, root, "rev-list", "--left-right", "--count", def+"...HEAD")
	fields := strings.Fields(counts)
	if err != nil || len(fields) != 2 {
		return "branch: " + current + state + " — distance from " + def + " unknown (" + gitReason(err) + ")"
	}
	return fmt.Sprintf("branch: %s%s — %s ahead, %s behind %s", current, state, fields[1], fields[0], def)
}

// rebaseState names an in-progress rebase, the way git's own directories
// report it: the state is a fact about the tree the next session inherits.
func rebaseState(gitDir string) string {
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(gitDir, d)); err == nil {
			return " (rebase in progress)"
		}
	}
	return ""
}

func headLine(ctx context.Context, root string) string {
	if h := shortHead(ctx, root); h != "" {
		return "head: " + h
	}
	return "head: unknown — no commits yet"
}

func shortHead(ctx context.Context, root string) string {
	h, err := git(ctx, root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return h
}

// dirtyLine counts what a commit would have to deal with, from git's own
// porcelain output — deletions included, because a deleted file is a change.
func dirtyLine(ctx context.Context, root string) string {
	out, err := git(ctx, root, "status", "--porcelain")
	if err != nil {
		return "dirty files: unknown — " + gitReason(err)
	}
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	if n == 0 {
		return "dirty files: none (clean tree)"
	}
	return fmt.Sprintf("dirty files: %d", n)
}

// gitReason turns a failed git call into the one line the unknown value
// carries. git writes the useful sentence to stderr; exec keeps it.
func gitReason(err error) string {
	if err == nil {
		return "reason unknown"
	}
	var stderr string
	if ee, ok := err.(*exec.ExitError); ok {
		stderr = string(ee.Stderr)
	}
	for _, line := range strings.Split(stderr, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return err.Error()
}

// --- the project layer -------------------------------------------------------

// sprintLines report the active sprint and what is still open inside it.
func sprintLines(root string) []string {
	items, err := backlog.LoadAll(root)
	if err != nil {
		return []string{"sprint: unknown — " + err.Error()}
	}
	if len(items) == 0 {
		return []string{"sprint: none — no backlog yet (`procoder backlog` starts one)"}
	}
	var active backlog.Item
	found := false
	for _, it := range items {
		if it.Kind != backlog.KindSprint {
			continue
		}
		if it.Status == "unreadable" {
			return []string{"sprint: unknown — sprint file " + it.ID + ".md could not be read (" + it.Title + ")"}
		}
		if it.Status == "active" {
			active, found = it, true
		}
	}
	if !found {
		return []string{"sprint: none active — `procoder sprint open <goal>` starts one"}
	}

	var open []backlog.Item
	total, done := 0, 0
	for _, it := range items {
		if it.Kind != backlog.KindStory || it.Sprint != active.ID {
			continue
		}
		total++
		if it.Done() {
			done++
			continue
		}
		open = append(open, it)
	}
	lines := []string{fmt.Sprintf("sprint: %s  %s — %d of %d stories done", active.ID, active.Title, done, total)}
	// this block is injected into every session: a sprint carrying dozens of
	// stories must not push the principles out of view, so the listing is
	// capped and the remainder is counted, never dropped in silence
	rest := 0
	if len(open) > storyCap {
		rest = len(open) - storyCap
		open = open[:storyCap]
	}
	for _, s := range open {
		if s.Status == "unreadable" {
			lines = append(lines, "  open story: "+s.ID+" — unreadable ("+s.Title+")")
			continue
		}
		lines = append(lines, "  open story: "+s.ID+"  "+s.Title)
	}
	if rest > 0 {
		lines = append(lines, fmt.Sprintf("  … %d more open story(s) — `procoder sprint status` lists them all", rest))
	}
	return lines
}

// storyCap is how many open stories the block names before it starts
// counting: enough to see what the sprint is about, short enough that the
// injected block stays readable.
const storyCap = 8

// namedCap is how many ids a count line names before it stops listing: enough
// to recognise the work, short enough to stay one line.
const namedCap = 5

// taskLine counts the open todo tasks and names the first few.
func taskLine(root string) string {
	tasks, err := todo.List(root)
	if err != nil {
		return "open tasks: unknown — " + err.Error()
	}
	var ids []string
	unreadable := 0
	for _, t := range tasks {
		switch t.Status {
		case "open":
			ids = append(ids, t.ID)
		case "unreadable":
			unreadable++
		}
	}
	line := "open tasks: none"
	if len(ids) > 0 {
		named := ids
		suffix := ""
		if len(named) > namedCap {
			named, suffix = named[:namedCap], ", …"
		}
		line = fmt.Sprintf("open tasks: %d — %s%s", len(ids), strings.Join(named, ", "), suffix)
	}
	if unreadable > 0 {
		line += fmt.Sprintf(" (%d task file(s) unreadable — counted as unknown)", unreadable)
	}
	return line
}

// lessonLine counts the lessons recorded with no adaptation behind them: the
// classes still open against us.
func lessonLine(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(lessons.Path)))
	if os.IsNotExist(err) {
		return "unlearned lessons: none — no ledger at " + lessons.Path
	}
	if err != nil {
		return "unlearned lessons: unknown — " + err.Error()
	}
	unlearned := 0
	for _, e := range lessons.Parse(string(raw)) {
		if e.Adaptation == "" || strings.HasPrefix(e.Adaptation, "<") {
			unlearned++
		}
	}
	return fmt.Sprintf("unlearned lessons: %d", unlearned)
}

// indexLine reports whether an index exists and whether it still matches
// HEAD. Freshness with no HEAD to compare against is unknown, and says so —
// a stale index that reads as current is exactly the lie this domain bans.
func indexLine(root, head string, timedOut bool) string {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(codeindex.Dir), "meta.json"))
	if os.IsNotExist(err) {
		return "index: none — `procoder index build` has not run here"
	}
	if err != nil {
		return "index: unknown — " + err.Error()
	}
	var m codeindex.Meta
	if json.Unmarshal(raw, &m) != nil || m.Commit == "" {
		return "index: present but its metadata could not be read — run `procoder index build`"
	}
	base := fmt.Sprintf("index: built at %s (%d files, %d symbols)", m.Commit, m.Files, m.Tags)
	switch {
	case timedOut:
		return base + " — freshness unknown (HEAD lookup ran past the budget)"
	case head == "":
		return base + " — freshness unknown (HEAD could not be read)"
	case head == m.Commit:
		return base + " — current"
	}
	return base + " — STALE, HEAD is " + head + "; run `procoder index build`"
}

// deferredLine names the suites the commit gate will not run here, and
// says nothing when there are none — a line on every session in a
// single-language repository is noise the reader learns to skip.
//
// The gate narrows only the runners that take a target list; the rest are
// CI's. Without this line that trade is invisible: a JavaScript commit
// passes the gate having never run its suite, and a green gate reads as a
// suite that passed.
func deferredLine(root string) string {
	d := testrun.Deferred(root)
	if len(d) == 0 {
		return ""
	}
	return "gate defers to CI: " + strings.Join(d, ", ") + " suite(s) — the gate narrows only go and pytest"
}
