package format

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func needs(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("%s not installed on this machine", tool)
	}
}

func TestUnformattedGoReturnsTheFormattedBytes(t *testing.T) {
	needs(t, "gofmt")
	dir := t.TempDir()
	p := write(t, dir, "a.go", "package main\nfunc  main( ){}\n")
	res := Check(p)
	if res.Verdict != Unformatted {
		t.Fatalf("verdict = %v, want Unformatted (reason %q)", res.Verdict, res.Reason)
	}
	if !strings.Contains(string(res.Formatted), "func main() {}") {
		t.Fatalf("formatted output does not contain the fixed code:\n%s", res.Formatted)
	}
	// P-CONTROL: the file itself must be untouched.
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), "func  main( ){}") {
		t.Fatal("the checker modified the file — it must never do that")
	}
}

func TestCleanGoIsClean(t *testing.T) {
	needs(t, "gofmt")
	dir := t.TempDir()
	p := write(t, dir, "a.go", "package main\n\nfunc main() {}\n")
	if res := Check(p); res.Verdict != Clean {
		t.Fatalf("verdict = %v, want Clean (reason %q)", res.Verdict, res.Reason)
	}
}

func TestFileTypeWithoutFormatterIsOutOfScopeNotClean(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "notes.txt", "hello\n")
	res := Check(p)
	if res.Verdict != OutOfScope {
		t.Fatalf("verdict = %v, want OutOfScope", res.Verdict)
	}
	if res.Reason == "" {
		t.Fatal("out-of-scope must carry a reason — silence reads as checked")
	}
}

// The distinction this whole design defends: a missing tool is Unchecked with
// a reason, never Clean. Simulated by pointing PATH at an empty directory.
func TestMissingToolIsUncheckedNeverClean(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "a.go", "package main\nfunc  main( ){}\n")
	t.Setenv("PATH", t.TempDir())
	res := Check(p)
	if res.Verdict != Unchecked {
		t.Fatalf("verdict = %v, want Unchecked", res.Verdict)
	}
	if !strings.Contains(res.Reason, "not installed") {
		t.Fatalf("reason %q does not say the tool is missing", res.Reason)
	}
}

// clang-format without a project .clang-format must NOT impose LLVM style —
// the file is out of scope with the reason said.
// C without a .clang-format used to be out of scope, which the gate counts
// as skipped and passes. So every C and C++ project that had not written a
// style file was formatted by nothing and told it was fine. clang-format
// takes a fallback style on the command line, which imposes nothing on
// disk, so there was never a reason to check nothing.
// proved by: restored NeedsProjectConfig on clang-format — the file goes
// back to OutOfScope and this test names the verdict.
func TestCWithoutProjectConfigIsStillFormatted(t *testing.T) {
	if _, err := exec.LookPath("clang-format"); err != nil {
		t.Skip("clang-format not installed: ", err)
	}
	dir := t.TempDir()
	p := write(t, dir, "a.c", "int  main( ){return 0;}\n")
	res := Check(p)
	if res.Verdict == OutOfScope {
		t.Fatalf("a C file must be formatted against the baseline, not skipped (reason %q)", res.Reason)
	}
	// It is unformatted here, and the formatted bytes come with the verdict
	// so the agent has something to write.
	if res.Verdict == Unformatted && len(res.Formatted) == 0 {
		t.Error("an unformatted verdict must carry the formatted result")
	}
}

// Caught by procoder's own dogfood CI: prettier "fixes" the commit template by
// stripping its leading blank lines — the exact lines the commit editor opens
// on for the author to type into. Functional whitespace is not unformatted.
func TestCommitTemplateWhitespaceIsFunctionalNotUnformatted(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".procoder", "github")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	p := write(t, sub, "COMMIT_TEMPLATE.md", "\n\n# guide\n")
	res := Check(p)
	if res.Verdict != OutOfScope {
		t.Fatalf("verdict = %v, want OutOfScope (reason %q)", res.Verdict, res.Reason)
	}
	if !strings.Contains(res.Reason, "functional") {
		t.Fatalf("reason %q does not explain the whitespace is functional", res.Reason)
	}
}

// D-OVERRIDE, on the tool that used to demand a config: a repository with
// its own .clang-format is formatted by that file, and the baseline
// fallback is not consulted. The fallback exists so an unconfigured repo
// is still checked, not so procoder's taste beats the project's.
// proved by: dropped --style=file from clang-format's args, leaving only
// the fallback — the project's 20-column limit is ignored, the output
// stays on one line, and this test names it.
func TestARepositoryClangFormatWinsOverTheBaseline(t *testing.T) {
	if _, err := exec.LookPath("clang-format"); err != nil {
		t.Skip("clang-format not installed: ", err)
	}
	dir := t.TempDir()
	// A style nothing would produce by default: a 20-column limit forces
	// the one-line function below to wrap, so the verdict distinguishes
	// "the project's file was read" from "a builtin style was used".
	write(t, dir, ".clang-format", "BasedOnStyle: LLVM\nColumnLimit: 20\n")
	p := write(t, dir, "a.c", "int main(int argc, char **argv, char **envp) { return argc; }\n")

	res := Check(p)
	if res.Verdict != Unformatted {
		t.Fatalf("the project's 20-column style must find this line too long, got %v (%s)", res.Verdict, res.Reason)
	}
	if !strings.Contains(string(res.Formatted), "\n") ||
		len(strings.Split(strings.TrimSpace(string(res.Formatted)), "\n")) < 3 {
		t.Errorf("the project's column limit must have wrapped the line:\n%s", res.Formatted)
	}
}
