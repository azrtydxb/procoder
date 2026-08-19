package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func stopRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "first")
	return root
}

func handoffPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(StateDir), HandoffFile)
}

func readHandoff(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(handoffPath(root))
	if err != nil {
		t.Fatalf("no handoff note written: %v", err)
	}
	return string(raw)
}

func shortHead(t *testing.T, root string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestStopWritesTheFactsBlockWithHead(t *testing.T) {
	root := stopRepo(t)
	if code := Stop(strings.NewReader(`{"hook_event_name":"Stop","cwd":"`+filepath.ToSlash(root)+`"}`), root); code != 0 {
		t.Fatalf("stop exit %d — a session ending must never fail", code)
	}
	note := readHandoff(t, root)
	for _, want := range []string{factsOpen, factsClose, "generated: ", "head: " + shortHead(t, root), "branch: main", notesHead} {
		if !strings.Contains(note, want) {
			t.Fatalf("handoff note is missing %q:\n%s", want, note)
		}
	}
	if strings.Index(note, factsOpen) > strings.Index(note, factsClose) {
		t.Fatalf("the facts markers are in the wrong order:\n%s", note)
	}
}

// An empty stdin is a host that sent nothing; the note is still written.
func TestStopToleratesAnEmptyPayload(t *testing.T) {
	root := stopRepo(t)
	if code := Stop(strings.NewReader(""), root); code != 0 {
		t.Fatalf("stop exit %d", code)
	}
	if !strings.Contains(readHandoff(t, root), factsOpen) {
		t.Fatal("no facts block after an empty payload")
	}
}

func TestAgentNotesSurviveTheNextStopWhileFactsUpdate(t *testing.T) {
	root := stopRepo(t)
	Stop(strings.NewReader("{}"), root)

	note := readHandoff(t, root)
	mine := "I was mid-way through the parser rewrite; next is the error path."
	if err := os.WriteFile(handoffPath(root), []byte(note+"\n"+mine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readHandoff(t, root)
	if !strings.Contains(before, "dirty files: none") {
		t.Fatalf("fixture assumption broken — the tree should be clean:\n%s", before)
	}

	// something changes in the repository, so the facts have to move
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Stop(strings.NewReader("{}"), root)

	after := readHandoff(t, root)
	if !strings.Contains(after, mine) {
		t.Fatalf("agent-authored notes were lost on rewrite:\n%s", after)
	}
	if strings.Contains(after, "dirty files: none") {
		t.Fatalf("the facts block did not update — the tree is no longer clean:\n%s", after)
	}
	if strings.Count(after, factsOpen) != 1 {
		t.Fatalf("the facts block was duplicated instead of replaced:\n%s", after)
	}
}

// The markers are the contract, but a human editing the file may delete them.
// The whole note is rewritten then, and the notes section it can still find is
// carried across — the agent's words are never the thing that gets dropped.
func TestHandDeletedMarkersRewriteTheFileAndKeepTheNotes(t *testing.T) {
	root := stopRepo(t)
	mine := "decision: keep the walker single-pass"
	if err := os.MkdirAll(filepath.Dir(handoffPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoffPath(root),
		[]byte("# procoder handoff\n\nsome stale hand-written facts\n\n"+notesHead+"\n\n"+mine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Stop(strings.NewReader("{}"), root)

	after := readHandoff(t, root)
	if !strings.Contains(after, factsOpen) || !strings.Contains(after, factsClose) {
		t.Fatalf("the markers were not restored:\n%s", after)
	}
	if !strings.Contains(after, mine) {
		t.Fatalf("the notes section was not preserved:\n%s", after)
	}
	if strings.Contains(after, "some stale hand-written facts") {
		t.Fatalf("stale facts above the notes must be replaced, not kept:\n%s", after)
	}
}

func TestUnwritableStateDirectoryExitsZeroSilently(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not deny writes on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root writes everywhere; the permission bit proves nothing")
	}
	root := stopRepo(t)
	dir := filepath.Join(root, ".procoder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("cannot drop write permission here")
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if code := Stop(strings.NewReader("{}"), root); code != 0 {
		t.Fatalf("stop exit %d — an unwritable state directory must never break a session", code)
	}
	if _, err := os.Stat(handoffPath(root)); err == nil {
		t.Fatal("a note was written into a directory that should have refused it")
	}
}
