package debt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoWith builds a git repository holding the given files, because Scan
// reads git's file list rather than walking the directory.
func repoWith(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const rots = "package x\n\n// debt: a global lock over the whole map.\nfunc f() {}\n"
const named = "package x\n\n// debt: a global lock; revisit when contention shows in a profile.\nfunc g() {}\n"

// The marker being added right now is the one worth saying something
// about, while the reason for it is still in the author's head. The whole
// ledger belongs to CI: eight markers printed on every commit is how a
// list becomes wallpaper.
// proved by: dropped the changed-file filter — every marker in the tree
// is reported on every commit, and the gate output becomes a ledger.
func TestAMarkerWithNoRevisitConditionIsReportedAtTheGate(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": rots, "b.go": named})

	got := GateCheck(root, []string{filepath.Join(root, "a.go")})
	if len(got) != 1 {
		t.Fatalf("the marker in the changed file, and only it: %v", got)
	}
	if !strings.Contains(got[0].Message, "a.go:3") {
		t.Errorf("the finding must name the file and line: %q", got[0].Message)
	}
	// Reported, not blocking: a deliberate shortcut is the author's call.
	// What is not their call is making it silently.
	if got[0].Blocking {
		t.Error("a debt marker is reported, not blocked on")
	}

	// A marker that names its revisit condition is doing what the rule
	// asks and says nothing.
	if got := GateCheck(root, []string{filepath.Join(root, "b.go")}); len(got) != 0 {
		t.Errorf("a marker with a condition is not a finding: %v", got)
	}
}

// The two tiers must not silently collapse into one. A marker in a file
// the commit did not touch is CI's — the gate answers about the change.
// proved by: matched on NoTrigger alone without consulting the changed
// set — an untouched file's marker blocks a commit that never saw it.
func TestAnUntouchedMarkerIsCIsNotTheGates(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": rots, "b.go": named})

	if got := GateCheck(root, []string{filepath.Join(root, "b.go")}); len(got) != 0 {
		t.Errorf("a.go's marker is not this commit's business: %v", got)
	}

	// And CI, which reads the whole tree, does see it — and now exits
	// non-zero, so the step can fail rather than printing into a log.
	var lines []string
	code := Run(root, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "a.go") {
		t.Errorf("the whole-tree pass reads every file: %s", joined)
	}
	if code == 0 {
		t.Error("rot in the tree must fail the CI step, not just print")
	}
}
