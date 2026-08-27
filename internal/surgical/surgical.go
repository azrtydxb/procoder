// Package surgical asks whether a change stayed where it said it would.
//
// The failure is drive-by edits: a change that does what was asked and
// also touches four files nowhere near it, each one a small unrelated
// improvement nobody reviewed as such. The principles already say a bug
// fix goes to the root cause and a small diff in the wrong place is a
// second bug — but that is prose, and prose does not notice (#197).
//
// A plan declares which files each task touches; `plan check` already
// blocks a task that names none. So the declared set exists, and the
// changed set is a git question. This compares them.
//
// Report-only, and quiet when it cannot tell. A plan is not required, most
// changes have none, and refusing on that basis would make every small
// commit argue with a checker. What it says is narrower and true: these
// files changed and nothing in the plan mentions them.
package surgical

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"procoder/internal/gitx"
)

// filesLine finds a plan task's file declaration. Both spellings, because
// both exist: the marketplace plan writes “**Files:** `a`, `b` “ and the
// backlog plan writes `Files: a, b` — plain, comma-separated, no ticks.
//
// The first version of this required backticks, and found nothing in three
// of the four plans in this repository while the line itself matched. It
// was caught by running the check against the real plans rather than
// against the template, which asks for one form and does not get it.
var (
	filesLine = regexp.MustCompile(`(?m)^\s*\**Files:\**\s*(.+)$`)
	ticked    = regexp.MustCompile("`([^`]+)`")
)

// paths splits a declaration into file names, accepting either spelling.
// Backticked entries win when present, because prose around them is
// common; otherwise the line is comma-separated paths.
func paths(decl string) []string {
	if m := ticked.FindAllStringSubmatch(decl, -1); len(m) > 0 {
		var out []string
		for _, t := range m {
			out = append(out, strings.TrimSpace(t[1]))
		}
		return out
	}
	var out []string
	for _, part := range strings.Split(decl, ",") {
		part = strings.TrimSpace(part)
		// Drop a trailing parenthetical or comment the author added.
		if i := strings.Index(part, " ("); i > 0 {
			part = part[:i]
		}
		part = strings.Trim(part, "`*")
		if part == "" || strings.HasPrefix(part, "<!--") {
			continue
		}
		out = append(out, part)
	}
	return out
}

// Declared is every file the plans say a task will touch.
//
// The union across tasks, not per task: a commit legitimately spans
// several tasks, and attributing a file to the wrong one would produce a
// finding about bookkeeping rather than about the change.
func Declared(root string, planFiles []string) (map[string]bool, int) {
	out := map[string]bool{}
	declaring := 0
	for _, p := range planFiles {
		// plan.Files hands back absolute paths; a caller with
		// repository-relative ones is equally valid. Joining root onto an
		// already-absolute path produced a path that does not exist, and
		// every plan was skipped in silence — which is why this reported
		// "no plan declares files" against four plans that do.
		full := p
		if !filepath.IsAbs(full) {
			full = filepath.Join(root, filepath.FromSlash(p))
		}
		raw, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		found := false
		for _, m := range filesLine.FindAllStringSubmatch(string(raw), -1) {
			for _, name := range paths(m[1]) {
				out[path.Clean(filepath.ToSlash(name))] = true
				found = true
			}
		}
		if found {
			declaring++
		}
	}
	return out, declaring
}

// Check reports changed files no plan mentions.
//
// Silence when there is nothing to compare against: no plan, or plans that
// declare no files. That is not a pass being handed out — there is
// genuinely no declared scope, and saying so is what the caller prints.
func Check(root string, changed []string, planFiles []string, out func(string)) int {
	declared, declaring := Declared(root, planFiles)
	if declaring == 0 {
		out("scope NOT checked — no plan declares files, so there is nothing to compare a change against " +
			"(a plan's `**Files:**` lines are the declaration; `procoder plan check` already blocks a task without one)")
		return 0
	}
	var stray []string
	for _, c := range changed {
		rel := c
		if r, ok := gitx.RepoRel(root, c); ok {
			rel = r
		}
		rel = path.Clean(filepath.ToSlash(rel))
		if declared[rel] || underDeclaredDir(declared, rel) {
			continue
		}
		stray = append(stray, rel)
	}
	sort.Strings(stray)
	if len(stray) == 0 {
		out(fmt.Sprintf("every changed file is in the declared scope (%d file(s) declared across the plans)", len(declared)))
		return 0
	}
	for _, s := range stray {
		out("  " + s + " — changed, and no plan names it")
	}
	out(fmt.Sprintf("%d file(s) outside the declared scope — report only. Either the plan is behind the work, or the work wandered; both are worth a look, neither is automatically wrong",
		len(stray)))
	return 0
}

// underDeclaredDir lets a plan name a directory and cover what is in it.
// A task that says `internal/gate/` has declared the files beneath it, and
// demanding each one by name would make the declaration a maintenance
// chore nobody keeps current.
func underDeclaredDir(declared map[string]bool, rel string) bool {
	for d := range declared {
		if strings.HasSuffix(d, "/") && strings.HasPrefix(rel, d) {
			return true
		}
		if !strings.Contains(path.Base(d), ".") && strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}
