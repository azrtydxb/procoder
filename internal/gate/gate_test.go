package gate

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateFailsOnUnformattedAndPassesAfter(t *testing.T) {
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skip("gofmt not installed")
	}
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
