package security

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubSemgrep puts a fake semgrep on PATH that prints the given JSON. A
// stub rather than the real scanner because what is under test is
// procoder's half — which files it asks about, and what it does with the
// answer. Requiring the real semgrep would make this skip on every runner
// that has not installed it, and the test job has not.
func stubSemgrep(t *testing.T, out string) {
	t.Helper()
	stubSemgrepAfter(t, out, 0)
}

// stubSemgrepAfter is stubSemgrep with a deliberate delay before the
// answer, for the tests that assert a slow check is still a completed
// one.
func stubSemgrepAfter(t *testing.T, out string, seconds int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	sleep := ""
	if seconds > 0 {
		sleep = fmt.Sprintf("sleep %d\n", seconds)
	}
	script := "#!/bin/sh\n" + sleep + "cat <<'JSON'\n" + out + "\nJSON\n"
	if err := os.WriteFile(filepath.Join(bin, "semgrep"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

const oneError = `{"results":[{"path":"bad.py","start":{"line":3},"check_id":"subprocess-shell-true","extra":{"severity":"ERROR","message":"shell=True is dangerous"}}]}`

// A SAST finding in a file the commit carries blocks it. Before this the
// scan ran only in CI, so the author learned after pushing.
// proved by: dropped SastChanged from the gate — the finding below is
// reported by nothing and the commit goes through.
func TestASastFindingInAChangedFileBlocks(t *testing.T) {
	stubSemgrep(t, oneError)
	root := t.TempDir()
	got := SastChanged(root, []string{filepath.Join(root, "bad.py")})
	if len(got) != 1 {
		t.Fatalf("want the finding, got %+v", got)
	}
	if !got[0].Blocking {
		t.Error("an ERROR finding must block at the default bar")
	}
	if got[0].Line != 3 || got[0].File != "bad.py" {
		t.Errorf("the finding must carry its place, got %s:%d", got[0].File, got[0].Line)
	}
}

// The scoping is applied to the FINDINGS, not to the scan's targets, and
// the difference is not cosmetic. Handing semgrep an explicit file list
// makes it scan files its own default selection skips: doing that flagged
// an exec.Command in a _test.go file that `security --deep` had never
// once reported, so a developer would have been blocked by a finding CI
// does not have. The scan is the same whole-tree scan; only its answers
// are narrowed to the commit.
// proved by: filtered on nothing and returned every finding — a finding
// in a file the commit never touched blocks it, which is the whole-repo
// verdict wearing the gate's name.
func TestOnlyFindingsInChangedFilesBlockTheCommit(t *testing.T) {
	const twoFiles = `{"results":[
      {"path":"mine.py","start":{"line":1},"check_id":"a","extra":{"severity":"ERROR","message":"in my change"}},
      {"path":"theirs.py","start":{"line":9},"check_id":"b","extra":{"severity":"ERROR","message":"somewhere else"}}
    ]}`
	stubSemgrep(t, twoFiles)
	root := t.TempDir()

	got := SastChanged(root, []string{filepath.Join(root, "mine.py")})
	if len(got) != 1 {
		t.Fatalf("only the finding in the changed file belongs to this commit: %+v", got)
	}
	if got[0].File != "mine.py" {
		t.Errorf("wrong finding kept: %+v", got[0])
	}

	// Narrowing must not mean dropping: a commit touching both owns both.
	both := SastChanged(root, []string{filepath.Join(root, "mine.py"), filepath.Join(root, "theirs.py")})
	if len(both) != 2 {
		t.Errorf("a commit touching both files owns both findings: %+v", both)
	}
}

// A finding with no path — a scan that could not run, output that could
// not be read — belongs to the commit whatever it touched. Dropping it
// would filter away the very reports that say the check did not happen.
// proved by: filtered on the path unconditionally — "semgrep is not
// installed" is discarded and the commit passes unscanned.
func TestAFindingWithNoPathIsNotFilteredAway(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\n" + "echo 'not json'\n" + "exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "semgrep"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	got := SastChanged(root, []string{filepath.Join(root, "a.py")})
	if len(got) != 1 {
		t.Fatalf("a scan that did not run must reach the commit: %+v", got)
	}
	if !got[0].Blocking {
		t.Error("a check that did not happen must block")
	}
}

// A commit that carries no files asks semgrep nothing, rather than
// scanning the tree by accident — the shape that would turn a docs-only
// commit into a full scan.
// proved by: dropped the empty check — an empty file list falls through to
// the whole-tree default.
func TestNoChangedFilesMeansNoScan(t *testing.T) {
	stubSemgrep(t, oneError)
	if got := SastChanged(t.TempDir(), nil); len(got) != 0 {
		t.Errorf("nothing changed means nothing to scan, got %+v", got)
	}
}

// A directory legitimately named "..foo" is inside the repository, and a
// path is the same file however it arrived — absolute from git, or
// relative because a person typed it. Both are gitx.RepoRel's job now;
// what is asserted here is that this leg uses it, because the failure is
// silent: the file drops out of the commit's set and its finding simply
// never blocks.
// proved by: went back to filepath.Rel on the raw path — the relative
// form yields nothing and `procoder check bad.py` stops being scanned
// while `procoder check` on the same file still is.
func TestAFindingIsMatchedHoweverThePathArrived(t *testing.T) {
	stubSemgrep(t, `{"results":[{"path":"..foo/a.py","start":{"line":1},"check_id":"x","extra":{"severity":"ERROR","message":"inside"}}]}`)
	root := t.TempDir()

	// Absolute, as git hands them over.
	if got := SastChanged(root, []string{filepath.Join(root, "..foo", "a.py")}); len(got) != 1 {
		t.Errorf("absolute path: a file under ..foo/ is in the repository: %+v", got)
	}
	// Relative, as a person types them.
	if got := SastChanged(root, []string{filepath.FromSlash("..foo/a.py")}); len(got) != 1 {
		t.Errorf("relative path: the same file must match: %+v", got)
	}
}

// A slow check still completes and still reports what it found. There is
// no budget on the heavy legs: cutting one off partway and printing the
// findings it happened to reach would make the verdict a fact about the
// machine rather than about the code, and the fast machine and the slow
// one would disagree about whether a commit is safe.
//
// The ceiling that does exist is a hung-process net, not a budget: when
// it fires the finding says SAST was NOT run, and blocks. Silence is
// never the answer.
// proved by: wrapped the scan in a 1-second context and returned the
// findings gathered so far — the slow run reports clean and the commit
// lands with the finding still in the file.
func TestASlowCheckStillCompletes(t *testing.T) {
	stubSemgrepAfter(t, oneError, 2)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	got := SastChanged(root, []string{filepath.Join(root, "bad.py")})
	waited := time.Since(start)

	if len(got) != 1 {
		t.Fatalf("a slow scanner's findings are still the answer: %v", got)
	}
	if !strings.Contains(got[0].Message, "shell=True") {
		t.Errorf("the finding must survive the wait intact: %q", got[0].Message)
	}
	// The gate waited rather than reporting a verdict it had not reached.
	if waited < 2*time.Second {
		t.Errorf("the gate returned before the check answered, in %s", waited)
	}
}

// The same commit produces the same findings however fast the machine
// is. This is the property a budget would take away, and it is asserted
// by comparing two runs that differ only in how long the scanner took.
// proved by: gave the scan a 1-second ceiling that returns what it has —
// the two runs disagree, and which one a developer gets depends on their
// laptop.
func TestFastAndSlowRunsAgree(t *testing.T) {
	scan := func(seconds int) []string {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "bad.py"), []byte("x = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stubSemgrepAfter(t, oneError, seconds)
		var out []string
		for _, f := range SastChanged(root, []string{filepath.Join(root, "bad.py")}) {
			out = append(out, fmt.Sprintf("%v|%s", f.Blocking, f.Message))
		}
		return out
	}
	fast, slow := scan(0), scan(2)
	if len(fast) == 0 {
		t.Fatal("the fixture must produce a finding, or the comparison proves nothing")
	}
	if !reflect.DeepEqual(fast, slow) {
		t.Errorf("the verdict must not depend on the machine:\n fast: %v\n slow: %v", fast, slow)
	}
}
