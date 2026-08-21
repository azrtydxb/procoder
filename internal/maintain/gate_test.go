package maintain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/gitx"
)

// unrun reports whether a finding is the leg saying it could not run,
// rather than a judgement about the code. There are two spellings — the
// per-leg "NOT checked" line and the summary "did NOT run" — and a test
// that knows only one passes where golangci-lint is installed and fails
// where it is not, which is the machine-dependent verdict this sprint
// exists to remove, arriving through the test suite.
func unrun(f gitx.Finding) bool {
	return strings.Contains(f.Message, "NOT checked") || strings.Contains(f.Message, "did NOT run")
}

// Complexity reaches the commit gate, narrowed to the files the commit
// carries — it used to run only when somebody typed `procoder maintain`.
// Reported, not blocking, unless the repository asked: these are
// judgement calls, and this repository's own cmd/procoder/main.go carries
// a function of 181 statements against a threshold of 50, so a blocking
// default would stop anyone committing to the file that needs the
// refactor most.
// proved by: passed block=true unconditionally — every commit touching a
// long function is refused, including the commit that would shorten it.
func TestComplexityIsReportedAtTheGateAndBlocksOnlyOnRequest(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "cmd", "procoder", "main.go")
	if _, err := os.Stat(target); err != nil {
		t.Skip("this test reads the repository's own long file: ", err)
	}

	got := ComplexityChanged(root, []string{target}, false)
	if len(got) == 0 {
		t.Skip("no complexity findings here — golangci-lint may be absent")
	}
	for _, f := range got {
		if unrun(f) {
			t.Skip("the complexity checker could not run: ", f.Message)
		}
		if f.Blocking {
			t.Errorf("complexity must not block by default: %q", f.Message)
		}
		if !strings.HasSuffix(f.File, "main.go") {
			t.Errorf("only findings in the changed file belong to the commit: %+v", f)
		}
	}

	// The repository can ask for them to block.
	for _, f := range ComplexityChanged(root, []string{target}, true) {
		if unrun(f) {
			continue
		}
		if !f.Blocking {
			t.Errorf("under block, a complexity finding must block: %q", f.Message)
		}
	}
}

// A commit that touches none of the flagged files gets none of their
// findings — the narrowing is the point, or every commit would carry the
// whole repository's complexity report.
// proved by: dropped the changed-file test — a one-line docs commit
// arrives with every long function in the tree attached.
func TestAnUnrelatedCommitCarriesNoComplexityFindings(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	quiet := filepath.Join(root, "internal", "textutil", "textutil.go")
	if _, err := os.Stat(quiet); err != nil {
		t.Skip("fixture file missing: ", err)
	}
	for _, f := range ComplexityChanged(root, []string{quiet}, false) {
		if unrun(f) {
			continue
		}
		if !strings.HasSuffix(f.File, "textutil.go") {
			t.Errorf("a finding from another file reached this commit: %+v", f)
		}
	}

	// And nothing changed means nothing asked.
	if got := ComplexityChanged(root, nil, false); len(got) != 0 {
		t.Errorf("no changed files means no complexity report: %+v", got)
	}
}

// A repository directory named "..foo" starts with two dots without
// stepping upward. runTool re-bases paths that golangci printed relative
// to its temp config, and a plain ".." prefix test catches "..foo" too —
// leaving a temp-dir-relative path the gate cannot match against the
// commit's files, so the finding is dropped in silence. That is the same
// edge case the gate's own path handling was fixed for, surviving on the
// side that produces the paths rather than the side that reads them.
// proved by: restored strings.HasPrefix(p, "..") in escapesUp — "..foo/a.go"
// is re-based against the temp directory and no longer matches.
func TestADirectoryNamedLikeAStepUpIsNotOne(t *testing.T) {
	for path, want := range map[string]bool{
		"..":                             true,
		filepath.FromSlash("../x.go"):    true,
		filepath.FromSlash("..foo/x.go"): false,
		filepath.FromSlash("a/b.go"):     false,
		"":                               false,
	} {
		if got := escapesUp(path); got != want {
			t.Errorf("escapesUp(%q) = %v, want %v", path, got, want)
		}
	}
}
