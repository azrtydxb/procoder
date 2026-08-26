package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func rungit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// numbered builds a file of n lines, so a finding's line number means
// something: "line 40 is far from the change on line 2".
func numbered(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("line\n")
	}
	return b.String()
}

// S-6: in somebody else's repository, a finding on a line this commit did
// not write is not this commit's to answer for.
//
// proved by: `if f.Line == 0 || touched[key]` → `if true` in NarrowToDiff
// (the pre-existing finding survives, want 0 got 1).
func TestNarrowToDiffDropsFindingsOnUntouchedLines(t *testing.T) {
	dir := repo(t)
	write(t, dir, "big.go", numbered(40))
	rungit(t, dir, "add", "-A")
	rungit(t, dir, "commit", "-q", "-m", "base")

	// Change line 2 only. The finding sits on line 40.
	body := strings.Split(numbered(40), "\n")
	body[1] = "changed"
	write(t, dir, "big.go", strings.Join(body, "\n"))
	rungit(t, dir, "add", "-A")

	pre := []Finding{{File: filepath.Join(dir, "big.go"), Line: 40, Message: "pre-existing", Blocking: true}}
	got := NarrowToDiff(dir, []string{filepath.Join(dir, "big.go")}, pre)
	if len(got) != 0 {
		t.Fatalf("a finding on an untouched line survived: %+v", got)
	}
}

// The other half of the same criterion: the narrowing must not swallow
// what the commit DID write, or it is a silent green.
//
// proved by: `out = append(out, f)` → `continue` in NarrowToDiff's loop
// (the added line's finding vanishes, want 1 got 0).
func TestNarrowToDiffKeepsFindingsOnAddedLines(t *testing.T) {
	dir := repo(t)
	write(t, dir, "big.go", numbered(40))
	rungit(t, dir, "add", "-A")
	rungit(t, dir, "commit", "-q", "-m", "base")

	body := strings.Split(numbered(40), "\n")
	body[39-1] = "the commit wrote this"
	write(t, dir, "big.go", strings.Join(body, "\n"))
	rungit(t, dir, "add", "-A")

	mine := []Finding{{File: filepath.Join(dir, "big.go"), Line: 39, Message: "mine", Blocking: true}}
	got := NarrowToDiff(dir, []string{filepath.Join(dir, "big.go")}, mine)
	if len(got) != 1 {
		t.Fatalf("a finding on a line this commit wrote was dropped: %+v", got)
	}
}

// A brand-new file is entirely this commit's, and git shows it in no diff
// until it is staged. Every line counts as written.
//
// proved by: delete the `if !tracked(...)  markWholeFile(...)` loop from
// addedLines (the untracked file's finding disappears, want 1 got 0).
func TestNarrowToDiffKeepsEverythingInANewFile(t *testing.T) {
	dir := repo(t)
	write(t, dir, "seed.txt", "seed\n")
	rungit(t, dir, "add", "-A")
	rungit(t, dir, "commit", "-q", "-m", "base")

	write(t, dir, "brand-new.go", numbered(40))
	f := []Finding{{File: filepath.Join(dir, "brand-new.go"), Line: 37, Message: "in a new file", Blocking: true}}
	got := NarrowToDiff(dir, []string{filepath.Join(dir, "brand-new.go")}, f)
	if len(got) != 1 {
		t.Fatalf("a finding in a wholly new file was dropped: %+v", got)
	}
}

// No silent green: when the diff cannot be read, "which lines are mine" is
// unknown, and unknown must never be reported as "none of them".
//
// proved by: `return all` → `return nil` in the touched == nil branch
// (want 1 got 0).
func TestNarrowToDiffReportsEverythingWhenTheDiffCannotBeRead(t *testing.T) {
	dir := t.TempDir() // deliberately NOT a git repository
	write(t, dir, "x.go", "secret\n")
	f := []Finding{{File: filepath.Join(dir, "x.go"), Line: 1, Message: "x", Blocking: true}}
	got := NarrowToDiff(dir, []string{filepath.Join(dir, "x.go")}, f)
	if len(got) != 1 {
		t.Fatalf("findings were dropped when the diff could not be read: %+v", got)
	}
}

// A finding with no line is about the file, not a line in it, so it cannot
// be placed inside or outside the diff — it stays.
//
// proved by: `f.Line == 0 ||` deleted from the condition (want 1 got 0).
func TestNarrowToDiffKeepsWholeFileFindings(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "a\n")
	rungit(t, dir, "add", "-A")
	rungit(t, dir, "commit", "-q", "-m", "base")
	write(t, dir, "a.txt", "b\n")
	rungit(t, dir, "add", "-A")

	f := []Finding{{File: filepath.Join(dir, "a.txt"), Line: 0, Message: "about the file", Blocking: true}}
	if got := NarrowToDiff(dir, []string{filepath.Join(dir, "a.txt")}, f); len(got) != 1 {
		t.Fatalf("a whole-file finding was dropped: %+v", got)
	}
}

// The hunk header states where the added side begins; counting from the
// wrong number silently shifts every line by the size of the deletion.
//
// proved by: `rest := header[i+1:]` → `header[strings.Index(header,"-")+1:]`
// (reads the removed side's start, and the kept/dropped set inverts).
func TestHunkStartReadsTheAddedSide(t *testing.T) {
	cases := map[string]int{
		"@@ -1,4 +7,9 @@":            7,
		"@@ -1 +3 @@":                3,
		"@@ -10,0 +11,2 @@ func x()": 11,
	}
	for header, want := range cases {
		if got := hunkStart(header); got != want {
			t.Errorf("hunkStart(%q) = %d, want %d", header, got, want)
		}
	}
}
