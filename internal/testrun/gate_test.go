package testrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A commit is asked only about the ecosystems it is written in. This
// repository carries a package.json with no test script, so the JS runner
// reports NOT run forever — and treating that as a check that failed
// blocked every Go commit here, which is how it was caught.
// proved by: dropped the ecosystemsOf filter — a Go commit is told the
// JavaScript suite did not run and cannot proceed.
func TestOnlyTheEcosystemsTheCommitIsWrittenInAreAsked(t *testing.T) {
	for files, want := range map[string]string{
		"a.go":      "go",
		"a.ts":      "js",
		"a.py":      "python",
		"a.rs":      "rust",
		"a.php":     "php",
		"Main.java": "java",
	} {
		got := ecosystemsOf([]string{files})
		if !got[want] {
			t.Errorf("%s must implicate %s, got %v", files, want, got)
		}
		if len(got) != 1 {
			t.Errorf("%s implicates one ecosystem, got %v", files, got)
		}
	}

	// A commit of prose implicates nothing, so no suite is run at all.
	if got := ecosystemsOf([]string{"README.md", "docs/x.md"}); len(got) != 0 {
		t.Errorf("a prose commit runs no suite, got %v", got)
	}
}

// The narrowing is by package, because the whole suite cold is a minute
// on this repository and one package is a second. A commit invalidates
// the cache for what it changed, so "whole suite" is the realistic cost
// rather than the warm two seconds a second run reports.
// proved by: returned nil from changedPackages — every runner is given
// the whole tree and a one-line change pays for all of it.
func TestTheRunIsNarrowedToTheChangedPackages(t *testing.T) {
	// t.TempDir, not a literal like "/repo": on Windows a rooted path with
	// no volume is not absolute, so gitx.RepoRel joins it onto the root a
	// second time and the test measures its own path arithmetic. The same
	// fixture mistake this repository has now made three times.
	root := t.TempDir()
	got, _ := changedPackages(root, []string{
		filepath.Join(root, "internal", "textutil", "a.go"),
		filepath.Join(root, "internal", "textutil", "b.go"),
		filepath.Join(root, "cmd", "procoder", "main.go"),
	})
	if len(got) != 2 {
		t.Fatalf("two directories, deduplicated: %v", got)
	}
	if got[0] != "internal/textutil" || got[1] != "cmd/procoder" {
		t.Errorf("packages in commit order: %v", got)
	}

	// A path arriving relative names the same package as one arriving
	// absolute — the rule gitx.RepoRel exists to hold.
	rel, _ := changedPackages(root, []string{filepath.FromSlash("internal/textutil/a.go")})
	if len(rel) != 1 || rel[0] != "internal/textutil" {
		t.Errorf("relative form must name the same package: %v", rel)
	}

	// Only files whose runner READS a target list. Run's contract narrows
	// the Go package list and the pytest targets and nothing else, so a
	// directory of .js or .md files handed over lands in an argv that
	// cannot take it — `go test ./.kilo/plugin` fails as a broken
	// invocation and reads as a failing suite.
	got, run := changedPackages(root, []string{
		filepath.Join(root, ".kilo", "plugin", "procoder.js"),
		filepath.Join(root, ".agents", "rules", "procoder.md"),
	})
	if len(got) != 0 {
		t.Errorf("only path-reading runners get targets: %v", got)
	}
	// And no suite runs: the alternative is a JavaScript commit paying for
	// the whole Go suite before its results are filtered away. The limit
	// is deferred to CI, not hidden.
	if run {
		t.Error("a commit with no path-scoped file runs no suite at the gate")
	}
}

// One ecosystem's directories at a time, never a mixture. Run hands the
// same list to every runner it detects and the list means different
// things to each, so a Python directory reaches `go test` as a package
// that does not exist. This repository has a stray __init__.py at its
// root, so a Go commit was enough to produce exactly that: "# .".
// proved by: returned the union instead of one ecosystem's directories —
// a commit spanning both hands "." to `go test` and the gate reports a
// failing suite that never ran.
func TestATargetListNeverMixesEcosystems(t *testing.T) {
	root := t.TempDir()
	goOnly, _ := changedPackages(root, []string{
		filepath.Join(root, "a", "x.go"), filepath.Join(root, "b", "y.go")})
	if len(goOnly) != 2 {
		t.Errorf("a Go commit narrows to its packages: %v", goOnly)
	}
	pyOnly, _ := changedPackages(root, []string{filepath.Join(root, "c", "x.py")})
	if len(pyOnly) != 1 {
		t.Errorf("a Python commit narrows to its directories: %v", pyOnly)
	}
	// Both: no list at all, and every runner keeps its native
	// whole-project granularity — slower, and correct.
	both, runBoth := changedPackages(root, []string{
		filepath.Join(root, "a", "x.go"), filepath.Join(root, "c", "y.py")})
	if both != nil {
		t.Errorf("a commit spanning both ecosystems passes no targets: %v", both)
	}
	// An empty list and "do not run" are different answers, and returning
	// only the list conflated them: the mixed commit — the one most worth
	// testing — ran no suite at all while this test still passed.
	if !runBoth {
		t.Error("a commit spanning both ecosystems still runs its suites, whole-project")
	}
}

// A suite that could not run blocks whatever the policy says. The policy
// governs whether a FAILING test stops a commit; "no answer" is not a
// verdict it has an opinion about — the same rule the rest of the gate
// follows for a missing tool.
// proved by: gave the NotRun branch Blocking: block — a repository on
// report never learns its runner is missing, and reads a green gate as a
// passing suite.
func TestASuiteThatCouldNotRunBlocksWhateverThePolicySays(t *testing.T) {
	// Built by the same function production uses, because the bug this
	// guards against lives in the seam between the two: notRun writes
	// "NOT run — " into Detail, and a fixture that spells Detail by hand
	// cannot see the gate saying it a second time.
	could := notRun(Result{Ecosystem: "go"}, "the go toolchain is not installed")
	f := findingFor(could, false)
	if f == nil || !f.Blocking {
		t.Fatalf("a suite that could not run must block even under report: %+v", f)
	}
	if strings.Count(f.Message, "NOT run") != 1 {
		t.Errorf("the refusal says the suite did not run once, not twice: %q", f.Message)
	}
	if !strings.Contains(f.Message, "the go toolchain is not installed") {
		t.Errorf("the refusal must carry the reason: %q", f.Message)
	}

	failed := findingFor(Result{Ecosystem: "go", Verdict: Fail, Detail: "1 failing"}, false)
	if failed == nil || failed.Blocking {
		t.Errorf("a failing suite must not block under report: %+v", failed)
	}
	if blocked := findingFor(Result{Ecosystem: "go", Verdict: Fail, Detail: "1 failing"}, true); !blocked.Blocking {
		t.Error("under block, a failing suite must block")
	}

	// A passing suite says nothing: a line per commit per ecosystem would
	// be noise the reader learns to skip.
	if got := findingFor(Result{Ecosystem: "go", Verdict: Pass, Detail: "pass (12 tests)"}, true); got != nil {
		t.Errorf("a passing suite is silent, got %+v", got)
	}
}

// A dependency bump carries no source file and is exactly the change a
// suite is best at catching. go.mod names Go without holding a line of
// it, and narrowing to the directory the manifest sits in would test one
// package and call the whole dependency proven — so it gets the whole
// project.
// proved by: dropped the manifest lookups — a go.mod-only commit
// implicates nothing, runs no suite, and reports a green gate.
func TestADependencyBumpRunsTheSuiteItCouldBreak(t *testing.T) {
	root := t.TempDir()
	bump := []string{filepath.Join(root, "go.mod"), filepath.Join(root, "go.sum")}

	if eco := ecosystemsOf(bump); !eco["go"] {
		t.Errorf("a go.mod change implicates Go: %v", eco)
	}
	pkgs, run := changedPackages(root, bump)
	if !run {
		t.Fatal("a dependency bump runs the suite")
	}
	if pkgs != nil {
		t.Errorf("and runs it whole-project, not narrowed to the manifest's directory: %v", pkgs)
	}

	// A manifest for a runner the gate does not narrow stays deferred:
	// picking it up would put the gate back to running a whole suite it
	// has already decided CI owns.
	if _, run := changedPackages(root, []string{filepath.Join(root, "package.json")}); run {
		t.Error("a package.json bump is CI's, not the gate's")
	}
}

// The gate runs only the suites it can narrow, and every other one is
// CI's. That trade is invisible from a green gate — a JavaScript commit
// passes having never run its suite — so the deferral has to be
// nameable. Silence when there is nothing to name: a line on every
// session in a single-language repository is noise.
// proved by: returned every detected ecosystem rather than the deferred
// ones — a Go repository is told its Go suite was deferred to CI on
// every commit, which is the opposite of true.
func TestTheGateNamesTheSuitesItLeavesToCI(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// A Go repository defers nothing: the gate runs what it narrows.
	write("go.mod", "module x\n")
	if d := Deferred(root); len(d) != 0 {
		t.Errorf("a Go repository has nothing deferred: %v", d)
	}

	// A package.json with no test script is not a deferred suite — it is
	// no suite. Naming it would invent a check that does not exist.
	write("package.json", `{"name":"x"}`)
	if d := Deferred(root); len(d) != 0 {
		t.Errorf("no test script is no suite, not a deferred one: %v", d)
	}

	write("package.json", `{"scripts":{"test":"vitest run"}}`)
	write("Cargo.toml", "[package]\nname=\"x\"\n")
	got := Deferred(root)
	if len(got) != 2 || got[0] != "rust" || got[1] != "js" {
		t.Errorf("both suites the gate cannot narrow, in reading order: %v", got)
	}
}

// The list of what the gate runs is derived from the table that decides
// it. Restating it in the report's sentence is how a message keeps saying
// "go and pytest" the day a third runner learns to take a target list —
// and a status line that is confidently wrong is worse than none.
// proved by: returned a fixed []string{"go", "python"} — adding a runner
// to pathScoped leaves the report naming the old pair.
func TestWhatTheGateRunsIsDerivedNotRestated(t *testing.T) {
	got := Narrowed()
	if len(got) != 2 || got[0] != "go" || got[1] != "python" {
		t.Fatalf("the ecosystems pathScoped names, deduplicated and sorted: %v", got)
	}

	// The real assertion: it tracks the table. .pyi and .py both say
	// python and must not produce it twice, and a new entry must appear.
	pathScoped[".zig"] = "zig"
	defer delete(pathScoped, ".zig")
	if got := Narrowed(); len(got) != 3 || got[2] != "zig" {
		t.Errorf("a new path-scoped runner must reach the report: %v", got)
	}
}
