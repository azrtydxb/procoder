package testrun

import (
	"fmt"
	"path"
	"strings"

	"procoder/internal/gitx"
)

// GateCheck is the commit gate's test leg: the suite, narrowed to the
// packages this commit touches.
//
// The narrowing is what makes it possible at all. Measured on this
// repository with a cold cache, the whole suite is 64 seconds and one
// package is one — and a commit invalidates the cache for whatever it
// changed, so "whole suite" is the realistic cost rather than the warm
// two seconds a second run reports. Giving the runner less to do is the
// answer; cutting it off partway is not, because a verdict that depends
// on how fast the machine is, is not a verdict about the code.
//
// The trade is stated rather than hidden: a change can break a test in a
// package it does not contain, and this leg will not see it. CI runs the
// whole suite, which is the tier that answers about the tree.
//
// Blocking only where the repository asked. `[test] policy = "block"` has
// always meant the close controllers; it means the gate too now.
func GateCheck(root string, files []string, block bool) []gitx.Finding {
	pkgs := changedPackages(root, files)
	if len(pkgs) == 0 {
		return nil
	}
	want := ecosystemsOf(files)
	if len(want) == 0 {
		return nil
	}
	var out []gitx.Finding
	for _, r := range Run(root, pkgs, false, "") {
		// Only the ecosystems this commit is written in. A repository that
		// carries a package.json with no test script reports the JS runner
		// as NOT run forever; treating that as a check that failed would
		// block every Go commit in this repository, which is where it was
		// caught. "This ecosystem has no suite here" and "the suite could
		// not run" are different answers, and the runner spells both the
		// same way.
		if !want[r.Ecosystem] {
			continue
		}
		if f := findingFor(r, block); f != nil {
			out = append(out, *f)
		}
	}
	return out
}

// findingFor turns one runner's verdict into a gate finding, or nil when
// there is nothing to say.
//
// A FAILING suite blocks only where the repository asked, because that is
// what `[test] policy` has always meant. A suite that could NOT run
// blocks regardless: the policy governs whether a failing test stops a
// commit, and "no answer" is not a verdict it has an opinion about — the
// same rule the rest of the gate follows for a missing tool.
func findingFor(r Result, block bool) *gitx.Finding {
	switch r.Verdict {
	case Fail:
		return &gitx.Finding{Blocking: block,
			Message: fmt.Sprintf("%s tests: %s (test)", r.Ecosystem, TrimmedDetail(r.Detail))}
	case NotRun:
		return &gitx.Finding{Blocking: true,
			Message: fmt.Sprintf("%s tests NOT run — %s (test)", r.Ecosystem, TrimmedDetail(r.Detail))}
	}
	return nil
}

// changedPackages maps the commit's files to the directories that hold
// them, which is what the runners take as targets. Deduplicated and
// ordered so the same commit produces the same argv twice running.
func changedPackages(root string, files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range files {
		rel, ok := gitx.RepoRel(root, f)
		if !ok {
			continue
		}
		dir := path.Dir(rel)
		if dir == "." {
			dir = "."
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	// A commit of only deleted files leaves directories that no longer
	// exist; the runners report that honestly rather than being guessed at
	// here.
	return out
}

// TrimmedDetail keeps a runner's one-liner short enough to read in a gate
// that already prints a lot.
func TrimmedDetail(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// ecosystemsOf reports which runners a commit implicates, by the
// languages of the files it carries. A change to a .go file says nothing
// about whether the JavaScript suite passes.
func ecosystemsOf(files []string) map[string]bool {
	byExt := map[string]string{
		".go": "go",
		".js": "js", ".jsx": "js", ".mjs": "js", ".cjs": "js",
		".ts": "js", ".tsx": "js", ".mts": "js", ".cts": "js",
		".py": "python", ".pyi": "python",
		".rs":   "rust",
		".php":  "php",
		".java": "java", ".kt": "java", ".kts": "java",
	}
	out := map[string]bool{}
	for _, f := range files {
		if eco, ok := byExt[strings.ToLower(path.Ext(path.Clean(strings.ReplaceAll(f, "\\", "/"))))]; ok {
			out[eco] = true
		}
	}
	return out
}
