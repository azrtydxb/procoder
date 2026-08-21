package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// stubSemgrep puts a fake semgrep on PATH that prints the given JSON. A
// stub rather than the real scanner because what is under test is
// procoder's half — which files it asks about, and what it does with the
// answer. Requiring the real semgrep would make this skip on every runner
// that has not installed it, and the test job has not.
func stubSemgrep(t *testing.T, out string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncat <<'JSON'\n" + out + "\nJSON\n"
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
