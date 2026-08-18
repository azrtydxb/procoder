package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsWorkflowFile(t *testing.T) {
	yes := []string{
		".github/workflows/ci.yml",
		"/repo/.github/workflows/release.yaml",
	}
	no := []string{
		"config.yml",
		".github/dependabot.yml",
		".github/workflows/README.md",
	}
	for _, p := range yes {
		if !IsWorkflowFile(p) {
			t.Errorf("%s should be a workflow file", p)
		}
	}
	for _, p := range no {
		if IsWorkflowFile(p) {
			t.Errorf("%s should NOT be a workflow file", p)
		}
	}
}

// A missing actionlint must be a blocking NOT-checked finding per file — the
// honesty rule; silence would read as a clean workflow.
func TestMissingActionlintIsNotSilence(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := Lint([]string{".github/workflows/ci.yml"})
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("got %+v", got)
	}
}

// A fake actionlint proves the output parser: file:line:col: message.
func TestLintParsesFindings(t *testing.T) {
	bin := t.TempDir()
	fake := "#!/bin/sh\n" +
		"echo 'wf.yml:3:9: property \"runs-onn\" is not defined [syntax-check]'\n" +
		"echo 'wf.yml:7:1: unexpected key [syntax-check]'\n" +
		"exit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "actionlint"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	got := Lint([]string{"wf.yml"})
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(got), got)
	}
	if got[0].Line != 3 || got[1].Line != 7 || !got[0].Blocking {
		t.Fatalf("parsed wrong: %+v", got)
	}
}

// Exit non-zero with unparseable output is a failure, never a clean run.
func TestFailureWithoutFindingsIsNotClean(t *testing.T) {
	bin := t.TempDir()
	fake := "#!/bin/sh\necho 'panic: something broke' >&2\nexit 3\n"
	if err := os.WriteFile(filepath.Join(bin, "actionlint"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	got := Lint([]string{"wf.yml"})
	if len(got) != 1 || !got[0].Blocking || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("got %+v", got)
	}
}

func TestNoWorkflowFilesNoWork(t *testing.T) {
	if got := Lint(nil); got != nil {
		t.Fatalf("no files should produce nothing, got %+v", got)
	}
}
