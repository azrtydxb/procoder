package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"procoder/internal/slot"
)

// Every test in this package takes SAST slots from a directory of its own,
// never the machine's: a developer's running scan must not stall the suite,
// and the suite must not hold slots a real gate is waiting for.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "procoder-sast-slots")
	if err != nil {
		panic(err)
	}
	sastSlotDir = func() string { return dir }
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// stubSemgrepArgs is a semgrep that records its argv, one per line, and
// answers with an empty report.
func stubSemgrepArgs(t *testing.T) (argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a POSIX shell script")
	}
	bin := t.TempDir()
	argsFile = filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + argsFile + "'\necho '{\"results\":[]}'\n"
	if err := os.WriteFile(filepath.Join(bin, "semgrep"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func readArgs(t *testing.T, file string) []string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("semgrep was never run: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// The gate scans the tree with one anchored --include per changed file and
// a bounded --jobs; the target stays `.` so semgrep's default selection
// still decides what is scannable (#310).
// proved by: went back to scanning `.` with no --include — the argv below
// loses both includes and the gate scans the whole repository again.
func TestTheGateScansOnlyTheChangedFiles(t *testing.T) {
	argsFile := stubSemgrepArgs(t)
	root := t.TempDir()
	SastChanged(root, []string{filepath.Join(root, "sub", "a.py"), "b.py"})
	got := readArgs(t, argsFile)
	want := []string{"scan", "--config", "auto", "--json", "--quiet", "--jobs=2",
		"--include=/b.py", "--include=/sub/a.py", "."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semgrep argv\n got: %q\nwant: %q", got, want)
	}
}

// The deep pass is the whole tree, still with bounded jobs.
// proved by: dropped the --jobs flag — semgrep takes 85% of the cores.
func TestTheDeepPassScansTheTree(t *testing.T) {
	argsFile := stubSemgrepArgs(t)
	Sast(t.TempDir())
	got := readArgs(t, argsFile)
	want := []string{"scan", "--config", "auto", "--json", "--quiet", "--jobs=2", "."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("semgrep argv\n got: %q\nwant: %q", got, want)
	}
}

// proved by: removed the leading slash — `a.py` also matches `sub/a.py`;
// removed the escaping — `x[1].py` matches `x1.py` and not itself.
func TestIncludePatternNamesExactlyOneFile(t *testing.T) {
	for in, want := range map[string]string{
		"a.py":             "/a.py",
		"sub/a.py":         "/sub/a.py",
		"sub/we ird[1].py": `/sub/we ird\[1\].py`,
		"star*?.go":        `/star\*\?.go`,
		"trailing  ":       `/trailing\ \ `,
	} {
		if got := includePattern(in); got != want {
			t.Errorf("includePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

// A backslash is a name character on unix and must be escaped; on Windows
// it is the separator, and ToSlash turns it into one, so the case does not
// exist there.
func TestIncludePatternEscapesABackslashInAName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a backslash is the path separator on Windows")
	}
	if got, want := includePattern(`back\slash.go`), `/back\\slash.go`; got != want {
		t.Errorf("includePattern = %q, want %q", got, want)
	}
}

// Past maxIncludes the argv would grow without bound; the tree is scanned
// instead and the findings filtered, as before.
// proved by: removed the cap — 501 --include flags reach semgrep.
func TestTooManyFilesScanTheTree(t *testing.T) {
	changed := map[string]bool{}
	for i := 0; i < maxIncludes; i++ {
		changed[fmt.Sprintf("d/f%d.py", i)] = true
	}
	if got := includesFor(changed); len(got) != maxIncludes {
		t.Fatalf("%d files are still scoped, got %d includes", len(changed), len(got))
	}
	changed["one/more.py"] = true
	if got := includesFor(changed); got != nil {
		t.Errorf("%d files must scan the tree, got %d includes", len(changed), len(got))
	}
}

// A gate that cannot get a slot within its budget did not check anything,
// and says so — blocking, never silent, and without running semgrep.
// proved by: made a busy slot fall through to the scan — the stub runs and
// the gate reports clean while another scan of the same repository holds
// the slot.
func TestABusySlotIsNotChecked(t *testing.T) {
	if !slot.Supported {
		t.Skip("no slots on this platform")
	}
	argsFile := stubSemgrepArgs(t)
	root := t.TempDir()
	abs, _ := filepath.EvalSymlinks(root)
	sum := sha256.Sum256([]byte(abs))
	release, err := slot.Acquire(context.Background(), sastSlotDir(), "sast-repo-"+hex.EncodeToString(sum[:8]), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	got := sast(root, nil, 500*time.Millisecond)
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT run") {
		t.Fatalf("a scan that never got a slot must block as NOT run, got %+v", got)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Error("semgrep ran while another scan held the repository's slot")
	}
}
