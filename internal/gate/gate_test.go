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

func TestGateFailsOnUnformattedAndPassesAfter(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
	stubGitleaks(t)
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
