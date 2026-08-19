package lint

import (
	"fmt"
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

// Copilot suggested the lazy file capture drops Windows drive paths; the
// lazy quantifier expands until the rest matches, so it does not — pinned.
func TestParseHandlesWindowsDrivePaths(t *testing.T) {
	got := parse(`C:\dir\file.go:12: something wrong (lint)`+"\n", false)
	if len(got) != 1 || got[0].File != `C:\dir\file.go` || got[0].Line != 12 {
		t.Fatalf("windows path must parse: %+v", got)
	}
}

// A linter that fails without findings must never read as clean.
func TestFailedRunReadsAsNotChecked(t *testing.T) {
	got := finishParse("", fmt.Errorf("exec format error"), "x.py", "ruff", false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "NOT checked") {
		t.Fatalf("failed run must surface: %+v", got)
	}
	// a timeout arrives as an error with empty output — same rule
	got = finishParse("", fmt.Errorf("ruff gave no answer in 2m0s"), "x.py", "ruff", false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "gave no answer") {
		t.Fatalf("timeout must surface: %+v", got)
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

func TestConfiglessTSIsOutOfScopeButJSGetsTheBaseline(t *testing.T) {
	root := t.TempDir()
	got := Files(root, []string{filepath.Join(root, "a.ts")}, false)
	if len(got) != 1 || !strings.Contains(got[0].Message, "out of scope") {
		t.Fatalf("configless TS must be labeled out of scope: %+v", got)
	}

	if tools.Resolve(Eslint, "") == "" {
		t.Skip("eslint not installed; baseline leg runs where it is")
	}
	bad := filepath.Join(root, "bad.js")
	os.WriteFile(bad, []byte("var x = 1\nif (y == 2) { console.log(x) }\n"), 0o644)
	got = Files(root, []string{bad}, false)
	joined := ""
	for _, f := range got {
		joined += f.Message + "\n"
	}
	for _, want := range []string{"no-var", "eqeqeq", "no-undef", "procoder baseline"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("baseline must flag %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "console") {
		t.Fatalf("runtime globals must not be no-undef noise:\n%s", joined)
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

// The repo's own golangci config always beats the procoder baseline.
func TestHasGolangciConfigDetectsEveryName(t *testing.T) {
	if hasGolangciConfig(t.TempDir()) {
		t.Fatal("empty repo must have no golangci config")
	}
	for _, name := range []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !hasGolangciConfig(root) {
			t.Errorf("%s not detected", name)
		}
	}
}

// The baseline text itself must stay a parseable golangci v2 config shape:
// version pinned, and every curated linter named.
func TestGolangciBaselineNamesTheCuratedSet(t *testing.T) {
	for _, want := range []string{`version: "2"`, "gosec", "gocritic", "errorlint", "unparam", "copyloopvar", "nilerr"} {
		if !strings.Contains(golangciBaseline, want) {
			t.Errorf("baseline lost %q", want)
		}
	}
}
