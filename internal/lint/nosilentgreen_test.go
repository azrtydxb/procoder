package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The rule this file exists to hold: a check that did not happen is
// BLOCKING. Domain 1 has always blocked on a missing gitleaks; domain 2
// printed the same sentence as `info` and let the gate exit 0, so an empty
// machine was indistinguishable from clean code — which is the single
// largest source of green gates over code nothing read.
//
// [lint] policy is deliberately not consulted: that setting decides whether
// a linter's JUDGEMENTS stop a commit, and "the linter never ran" is not a
// judgement. The false argument here is block=false.
// proved by: dropped `Blocking: true` from notChecked — a repository with
// no linters installed goes back to passing `procoder check` with a list of
// info lines nobody has to act on.
func TestAMissingLinterBlocksRegardlessOfPolicy(t *testing.T) {
	got := notChecked("/x/a.go", "golangci-lint")
	if len(got) != 1 {
		t.Fatalf("want one finding, got %+v", got)
	}
	if !got[0].Blocking {
		t.Error("a linter that could not run must block the gate")
	}
	if !strings.Contains(got[0].Message, "procoder init") {
		t.Errorf("a blocking refusal needs a remedy: %q", got[0].Message)
	}
}

// Every extension procoder formats must reach a linter or be told plainly
// that none ran. .mjs and .cjs linted while .mts and .cts did not, and .pyi
// was formatted and never linted — silence by omission, not by decision.
// proved by: removed .mts/.cts from the dispatch — they produce no finding
// at all here, which is exactly how they passed before.
func TestEveryFormattedExtensionReachesALinterOrSaysItDoesNot(t *testing.T) {
	root := t.TempDir()
	// PATH emptied so no real linter answers: what is asserted is that the
	// file REACHES a linter, which shows up as a NOT-checked refusal naming
	// the tool rather than as nothing at all.
	t.Setenv("PATH", t.TempDir())
	for _, name := range []string{"a.mts", "a.cts", "a.pyi", "a.cpp", "a.c", "a.cs", "a.dart"} {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := Files(root, []string{p}, false)
		if len(got) == 0 {
			t.Errorf("%s reaches no linter and reports nothing — a silent pass", name)
			continue
		}
		// Two acceptable outcomes, and nothing else: a linter ran and had
		// something to say about this deliberately invalid fixture, or it
		// could not run and said THAT, blocking. What must never happen is
		// silence. The distinction is not asserted per extension because
		// which one you get depends on what is installed on the machine
		// running the suite — clang-tidy resolves from Homebrew's keg even
		// with PATH emptied, so C and C++ really are analysed here.
		for _, f := range got {
			if strings.Contains(f.Message, "NOT checked") || strings.Contains(f.Message, "NOT linted") {
				if !f.Blocking {
					t.Errorf("%s: a check that did not happen must block: %q", name, f.Message)
				}
			}
		}
	}
}

// The change must not swallow every unknown file. Out of scope is still a
// real verdict for a file type procoder does not claim — a text file, an
// image — and those must stay silent and green, or the gate becomes noise
// and gets switched off.
// proved by: made lintUnlinted report on any extension it did not
// recognise — a README or a .txt then blocks the gate.
func TestAFileTypeProcoderDoesNotClaimStaysSilent(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"notes.txt", "logo.png", "data.csv"} {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := Files(root, []string{p}, false); len(got) != 0 {
			t.Errorf("%s is not a file type procoder claims; it must stay silent: %+v", name, got)
		}
	}
}
