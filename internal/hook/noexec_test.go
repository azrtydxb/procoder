package hook_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file an agent session could have written is never auto-executed.
//
// procoder reads plenty of agent-written state — .procoder/ask/*,
// .procoder/state/handoff.md, the backlog, the specs — and hooks run
// unattended on every write, every commit and every session end. The one
// surface that executes a command the REPOSITORY declared is `procoder run
// --exec`, which displays by default and refuses outright when more than
// one candidate exists rather than guessing.
//
// This test is what keeps those two facts apart. A hook that reached the
// executing path would turn "procoder read a file an agent wrote" into
// "procoder ran what an agent wrote", unattended, which is the whole
// exposure (#201).
//
// Asserted by reading the sources rather than by running them: a
// behavioural test proves only the paths it exercised, and what is wanted
// here is absence from all of them.
//
// proved by: an import of procoder/internal/runcmd added to any file in
// internal/hook — the test names the file.
func TestNoHookExecutesWhatTheRepositoryDeclared(t *testing.T) {
	dir := "."
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the hook package could not be read: %v", err)
	}
	checked := 0
	for _, e := range files {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checked++
		body := string(raw)
		if strings.Contains(body, "procoder/internal/runcmd") {
			t.Errorf("internal/hook/%s reaches runcmd — a hook must never execute a command the repository declared", name)
		}
		if strings.Contains(body, `"os/exec"`) {
			t.Errorf("internal/hook/%s imports os/exec directly — every external process a hook needs goes through a domain with a fixed tool table", name)
		}
	}
	// An empty set would pass while proving nothing.
	if checked == 0 {
		t.Fatal("no hook source files were read — this test proved nothing")
	}
}
