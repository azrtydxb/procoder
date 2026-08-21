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
	// Only the ecosystems whose runners take a target list — Go and
	// pytest — are tested at the gate. The others run whole-project, and
	// running one of those here would mean a JavaScript commit paying for
	// the entire Go suite before its results were filtered away.
	//
	// So a commit that touches only Rust, PHP, Java or JavaScript is not
	// tested here. That is a real limit, and it is deferred rather than
	// hidden: CI runs every suite over the whole tree, and `procoder
	// status` names what the gate did not run.
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
		if r.NoSuite {
			// Nothing to run is not a check that failed to answer. A
			// repository with a package.json and no test script has no JS
			// suite, and saying so on every commit would block them all.
			return nil
		}
		return &gitx.Finding{Blocking: true,
			Message: fmt.Sprintf("%s tests NOT run — %s (test)", r.Ecosystem, TrimmedDetail(r.Detail))}
	}
	return nil
}

// changedPackages maps the commit's files to the directories the runners
// take as targets.
//
// One ecosystem's directories at a time, never a mixture. Run hands the
// same list to every runner it detects, and the list means different
// things to each: a Python directory reaches `go test` as a package that
// does not exist, which fails as "# ." and reads as a failing suite. This
// repository has a stray __init__.py at its root, so a Go commit was
// enough to produce exactly that.
//
// When a commit spans both, no list is passed and every runner keeps its
// native whole-project granularity — slower, and correct, which is the
// right way round.
func changedPackages(root string, files []string) []string {
	dirs := map[string][]string{}
	seen := map[string]bool{}
	for _, f := range files {
		rel, ok := gitx.RepoRel(root, f)
		if !ok {
			continue
		}
		eco, ok := pathScoped[strings.ToLower(path.Ext(rel))]
		if !ok {
			continue
		}
		dir := path.Dir(rel)
		key := eco + "\x00" + dir
		if seen[key] {
			continue
		}
		seen[key] = true
		dirs[eco] = append(dirs[eco], dir)
	}
	if len(dirs) != 1 {
		return nil
	}
	for _, d := range dirs {
		return d
	}
	return nil
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

// pathScoped are the file types whose runners accept a target list. Run
// narrows only the Go package list and the pytest targets; handing any
// other directory over would put it in an argv that cannot take it.
// pathScoped are the file types whose runners accept a target list, and
// which runner each belongs to. Run narrows only the Go package list and
// the pytest targets; every other runner ignores the list.
var pathScoped = map[string]string{".go": "go", ".py": "python", ".pyi": "python"}
