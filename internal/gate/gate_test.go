package gate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// stubGitleaks puts a fake gitleaks on PATH that reports no leaks, so the
// formatting-focused gate tests run on machines without the real scanner —
// the missing-scanner-blocks behavior has its own test in internal/security.
func stubGitleaks(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	if runtime.GOOS == "windows" {
		script := "@echo off\r\necho [] > %6\r\n"
		if err := os.WriteFile(filepath.Join(bin, "gitleaks.cmd"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	} else {
		script := "#!/bin/sh\nprintf '[]' > \"$6\"\n"
		if err := os.WriteFile(filepath.Join(bin, "gitleaks"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// stubLinter puts a golangci-lint on PATH that finds nothing, so a gate
// test about FORMATTING is not decided by whether a linter happens to be
// installed. It has to be here at all because a linter that could not run
// now blocks: the fixture is a lone .go file in a temp directory with no
// go.mod, where the real golangci-lint fails, and "no linter ran" is no
// longer the same answer as "the code is clean".
func stubLinter(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	name, script := "golangci-lint", "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		name, script = "golangci-lint.cmd", "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// stubSemgrep puts a semgrep on PATH that finds nothing, so a gate test
// about FORMATTING is not decided by whether a scanner happens to be
// installed. Needed since the gate gained its SAST leg: without it these
// tests pass on a machine with semgrep and fail on one without, which is
// the machine-dependent verdict the no-budget rule exists to prevent —
// arriving through the test suite instead of the gate.
func stubSemgrep(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	name, script := "semgrep", "#!/bin/sh\necho '{\"results\":[]}'\n"
	if runtime.GOOS == "windows" {
		name, script = "semgrep.cmd", "@echo off\r\necho {\"results\":[]}\r\n"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestGateFailsOnUnformattedAndPassesAfter(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	stubGitleaks(t)
	stubLinter(t)
	stubSemgrep(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	if err := os.WriteFile(p, []byte("package main\nfunc  main( ){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Run([]string{p}, dir, &out); code != 1 {
		t.Fatalf("exit %d for an unformatted file, want 1\n%s", code, out.String())
	}
	if err := os.WriteFile(p, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := Run([]string{p}, dir, &out); code != 0 {
		t.Fatalf("exit %d for a formatted file, want 0\n%s", code, out.String())
	}
}

func TestUncheckedFailsTheGateLikeUnformatted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	if err := os.WriteFile(p, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // no gofmt anywhere
	var out bytes.Buffer
	if code := Run([]string{p}, dir, &out); code != 1 {
		t.Fatalf("exit %d, want 1 — a file the gate could not look at is not a passing file", code)
	}
	if !strings.Contains(out.String(), "UNCHECKED") {
		t.Fatalf("output does not say the file was unchecked:\n%s", out.String())
	}
}

func TestOutOfScopeIsCountedNotFailed(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(p, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := Run([]string{p}, dir, &out); code != 0 {
		t.Fatalf("exit %d, want 0\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "1 out of scope") {
		t.Fatalf("skip was not counted out loud:\n%s", out.String())
	}
}

// The regression that live testing caught: five blocking hygiene findings were
// printed and the gate exited 0, because the exit condition only knew about
// formatting. A gate whose report and exit code disagree is worse than either
// alone — CI reads the exit, humans read the report.
func TestBlockingHygieneFailsTheExitCodeNotJustTheReport(t *testing.T) {
	stubGitleaks(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "conflicted.txt")
	content := "ok\n<<<<<<< HEAD\nmine\n=======\ntheirs\n>>>>>>> other\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := Run([]string{p}, dir, &out)
	if !strings.Contains(out.String(), "BLOCKING") {
		t.Fatalf("the conflict marker was not reported:\n%s", out.String())
	}
	if code != 1 {
		t.Fatalf("exit %d with a blocking finding in the report, want 1\n%s", code, out.String())
	}
}

// A repository with no test setup, no manifests and no rule files commits
// without a blocking finding. This sprint gave the gate five new legs —
// SAST, complexity, vulnerable dependencies, the suite, and agents drift
// — and each one had to decide what to say about a repository that has
// nothing for it to look at. "Nothing here" must come out silent; a
// repository that adopts procoder and cannot commit a text file has been
// told its empty tree is broken.
//
// The scanners are stubbed clean because what is under test is procoder's
// half. A machine WITHOUT them blocks, loudly and on purpose, and that
// has its own test in internal/security.
// proved by: made portability.AgentsDrift block when AGENTS.md is absent
// instead of returning nil — a repository that never opted into an agent
// layer is blocked for not having one.
func TestAQuietRepositoryStillCommits(t *testing.T) {
	stubGitleaks(t)
	stubSemgrep(t)

	root := t.TempDir()
	// A note and nothing else: no go.mod, no package.json, no AGENTS.md,
	// no pytest.ini, no lockfile.
	note := filepath.Join(root, "NOTES.txt")
	if err := os.WriteFile(note, []byte("A note.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	code := RunWith([]string{note}, root, "docs: a note", &buf)
	if code != 0 {
		t.Errorf("a quiet repository commits, got exit %d:\n%s", code, buf.String())
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, "BLOCKING") {
			t.Errorf("nothing here is not a finding: %s", line)
		}
	}
}
