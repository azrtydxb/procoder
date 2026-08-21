package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// The scan is given the changed files, not the tree. Scoping does not make
// semgrep cheap — its cost is rule loading, fixed at seconds — but on a
// large repository the difference between "the files in this commit" and
// "everything" is the part that keeps growing.
// proved by: passed "." instead of the file list — the argv no longer
// names the file, and every commit scans the whole tree.
func TestTheScanIsGivenTheChangedFiles(t *testing.T) {
	// The stub records its arguments so the argv can be asserted without
	// depending on what a real scanner would find in them.
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	argsFile := filepath.Join(bin, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\necho '{\"results\":[]}'\n"
	if err := os.WriteFile(filepath.Join(bin, "semgrep"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	SastChanged(root, []string{filepath.Join(root, "a.py"), filepath.Join(root, "b.py")})
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	argv := string(raw)
	for _, want := range []string{"a.py", "b.py"} {
		if !strings.Contains(argv, want) {
			t.Errorf("the scan must be given %s:\n%s", want, argv)
		}
	}
	// "." would be the whole tree, which is what this replaces.
	for _, line := range strings.Split(strings.TrimSpace(argv), "\n") {
		if line == "." {
			t.Errorf("the whole tree must not be scanned at the gate:\n%s", argv)
		}
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
