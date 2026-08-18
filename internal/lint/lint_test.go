package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"procoder/internal/tools"
)

func TestParseKeepsFindingsAndDropsExcerpts(t *testing.T) {
	raw := `internal/gitx/gitx_test.go:41:14: Error return value not checked (errcheck)
	os.WriteFile(p, []byte("x"), 0o644)
	            ^
script.sh:3:1: warning: quote this [SC2086]
m.py:7:5: F401 imported but unused
garbage line
`
	got := parse(raw, false)
	if len(got) != 3 {
		t.Fatalf("want 3 findings, got %d: %+v", len(got), got)
	}
	if got[0].File != "internal/gitx/gitx_test.go" || got[0].Line != 41 {
		t.Fatalf("wrong first finding: %+v", got[0])
	}
	for _, f := range got {
		if f.Blocking {
			t.Fatal("report mode must not block")
		}
	}
	if b := parse(raw, true); !b[0].Blocking {
		t.Fatal("block mode must block")
	}
}

func TestMissingLinterReadsAsNotCheckedNeverClean(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // keep the conventional-dir fallback empty too
	got := Files("/tmp", []string{"/tmp/x.py"}, false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("missing ruff must read NOT checked: %+v", got)
	}
}

func TestNoEslintConfigIsOutOfScopeNotSilent(t *testing.T) {
	root := t.TempDir()
	got := Files(root, []string{filepath.Join(root, "a.ts")}, false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "out of scope") {
		t.Fatalf("configless JS/TS must be labeled out of scope: %+v", got)
	}
}

func TestShellcheckFindsARealProblem(t *testing.T) {
	if tools.Resolve(Shellcheck, "") == "" {
		t.Skip("shellcheck not installed; parser tests carry the logic")
	}
	root := t.TempDir()
	f := filepath.Join(root, "bad.sh")
	os.WriteFile(f, []byte("#!/bin/sh\nrm -rf $1\n"), 0o755)
	got := Files(root, []string{f}, false)
	if len(got) == 0 {
		t.Fatal("shellcheck must flag the unquoted variable")
	}
	found := false
	for _, g := range got {
		if strings.Contains(g.Message, "SC2086") || strings.Contains(strings.ToLower(g.Message), "quote") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want the quoting finding, got %+v", got)
	}
}

func TestRuffFindsARealProblem(t *testing.T) {
	if tools.Resolve(Ruff, "") == "" {
		t.Skip("ruff not installed; parser tests carry the logic")
	}
	root := t.TempDir()
	f := filepath.Join(root, "m.py")
	os.WriteFile(f, []byte("import os\n\nprint('hi')\n"), 0o644)
	got := Files(root, []string{f}, false)
	if len(got) == 0 || !strings.Contains(got[0].Message, "F401") {
		t.Fatalf("ruff must flag the unused import: %+v", got)
	}
}
