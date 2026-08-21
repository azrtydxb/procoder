package testrun

import (
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
	root := filepath.FromSlash("/repo")
	got := changedPackages(root, []string{
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
	rel := changedPackages(root, []string{filepath.FromSlash("internal/textutil/a.go")})
	if len(rel) != 1 || rel[0] != "internal/textutil" {
		t.Errorf("relative form must name the same package: %v", rel)
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
	notRun := findingFor(Result{Ecosystem: "go", Verdict: NotRun,
		Detail: "the go toolchain is not installed"}, false)
	if notRun == nil || !notRun.Blocking {
		t.Fatalf("a suite that could not run must block even under report: %+v", notRun)
	}
	if !strings.Contains(notRun.Message, "NOT run") {
		t.Errorf("the refusal must say the suite did not run: %q", notRun.Message)
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
