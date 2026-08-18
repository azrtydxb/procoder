// Package ciops is domain 7: CI/CD hygiene for GitHub Actions workflows —
// the deterministic rules a senior developer holds pipelines to. actionlint
// (domain 9) covers syntax; this covers discipline: pinned actions, job
// timeouts, concurrency cancellation, and the existence of a test job.
// Everything reports except unpinned actions when the repo opted into
// blocking ([ci] pin_actions_policy = "block").
package ciops

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"procoder/internal/gitx"
)

// usesRe captures the action ref in a `uses:` line; group 2 is the ref.
var usesRe = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*([^@\s]+)@(\S+)`)

var shaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// testWordRe finds "test" as a word start — "ubuntu-latest" must not count
// as continuous testing.
var testWordRe = regexp.MustCompile(`(?i)\btest`)

// jobRe matches a job key: exactly two spaces of indentation under jobs:.
var jobRe = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):\s*$`)

// Check runs the workflow-hygiene rules over .github/workflows.
// pinBlock makes unpinned action refs blocking.
func Check(root string, pinBlock bool) []gitx.Finding {
	dir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // no workflows, nothing to hold to CI discipline
	}
	var out []gitx.Finding
	sawTestWord := false
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if e.IsDir() || (ext != ".yml" && ext != ".yaml") {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			out = append(out, gitx.Finding{File: path,
				Message: "workflow unreadable — NOT checked (ci)"})
			continue
		}
		text := string(raw)
		if testWordRe.MatchString(text) {
			sawTestWord = true
		}
		out = append(out, checkWorkflow(path, text, pinBlock)...)
	}
	if !sawTestWord {
		out = append(out, gitx.Finding{
			Message: "no workflow mentions tests — continuous testing is the CT in CI/CD/CT (ci)"})
	}
	return out
}

func checkWorkflow(path, text string, pinBlock bool) []gitx.Finding {
	var out []gitx.Finding
	lines := strings.Split(text, "\n")

	// unpinned action refs: a tag or branch can be repointed by its owner —
	// a supply-chain door; a 40-hex SHA cannot
	for i, line := range lines {
		m := usesRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		action, ref := m[1], m[2]
		if strings.HasPrefix(action, "./") || strings.HasPrefix(action, ".\\") {
			continue // local actions have no remote ref to pin
		}
		if !shaRe.MatchString(ref) {
			out = append(out, gitx.Finding{File: path, Line: i + 1, Blocking: pinBlock,
				Message: "action pinned to a mutable ref (" + action + "@" + ref + ") — a tag can be silently repointed; pin the commit SHA (ci)"})
		}
	}

	// per-job timeout-minutes: a hung job without one holds the runner for
	// GitHub's six-hour default
	inJobs := false
	jobStart := map[string]int{}
	var jobOrder []string
	current := ""
	jobBodies := map[string][]string{}
	for i, line := range lines {
		if strings.HasPrefix(line, "jobs:") {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && line[0] != '#' {
			inJobs = false // a new top-level key ends the jobs block
			continue
		}
		if m := jobRe.FindStringSubmatch(line); m != nil {
			current = m[1]
			jobStart[current] = i + 1
			jobOrder = append(jobOrder, current)
			continue
		}
		if current != "" {
			jobBodies[current] = append(jobBodies[current], line)
		}
	}
	sort.Strings(jobOrder)
	for _, job := range jobOrder {
		body := strings.Join(jobBodies[job], "\n")
		if !strings.Contains(body, "timeout-minutes:") {
			out = append(out, gitx.Finding{File: path, Line: jobStart[job],
				Message: "job \"" + job + "\" has no timeout-minutes — a hang holds the runner for GitHub's six-hour default (ci)"})
		}
	}

	// concurrency cancellation: without it, pushes pile up runs of stale code
	if !strings.Contains(text, "concurrency:") || !strings.Contains(text, "cancel-in-progress") {
		out = append(out, gitx.Finding{File: path,
			Message: "no concurrency group with cancel-in-progress — stacked pushes run CI on stale commits (ci)"})
	}
	return out
}
