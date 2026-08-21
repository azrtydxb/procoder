package maintain

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"procoder/internal/config"
	"procoder/internal/lint"
	"procoder/internal/tools"
)

// A deliberately over-complex function must show up; the report never exits
// non-zero — maintainability is judgment, not a gate.
func TestComplexityIsReportedAndNothingBlocks(t *testing.T) {
	if tools.Resolve(lint.GolangciLint, "") == "" {
		t.Skip("golangci-lint not installed")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644)

	// branch-heavy enough to trip gocyclo's default threshold
	var b strings.Builder
	b.WriteString("package demo\n\nfunc Tangled(x int) int {\n\tswitch {\n")
	for i := 0; i < 15; i++ {
		b.WriteString("\tcase x == " + strings.Repeat("1", i+1) + ":\n\t\treturn " + strings.Repeat("2", i+1) + "\n")
	}
	b.WriteString("\t}\n\tif x > 0 {\n\t\tif x > 1 {\n\t\t\tif x > 2 {\n\t\t\t\treturn 1\n\t\t\t}\n\t\t}\n\t}\n\treturn 0\n}\n")
	os.WriteFile(filepath.Join(root, "demo.go"), []byte(b.String()), 0o644)

	var lines []string
	code := Run(root, func(s string) { lines = append(lines, s) })
	if code != 0 {
		t.Fatalf("maintain must never exit non-zero, got %d\n%s", code, strings.Join(lines, "\n"))
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "complexity") || !strings.Contains(joined, "Tangled") {
		t.Fatalf("the tangled function must be reported:\n%s", joined)
	}
	if !strings.Contains(joined, "none blocking") {
		t.Fatalf("the summary must state nothing blocks:\n%s", joined)
	}
}

// maintain is report-only about its FINDINGS: complexity and dead code are
// judgement calls, nobody is blocked by them, and it exits 0 with a list.
//
// A check that could not run is not a finding. This test used to require
// exit 0 there too, which meant a machine without golangci-lint ran
// `procoder maintain`, printed "NOT checked", exited 0, and looked exactly
// like a clean report — and once CI runs this command, that is a job that
// passes because the tool was absent. Report-only describes the verdict on
// the code, not the question of whether the code was read.
// proved by: returned 0 from Run when a leg could not run — the missing
// tool is still printed and the command still succeeds, which is the
// silent green this whole rule exists to remove.
func TestMissingToolsSayNotCheckedAndFail(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n"), 0o644)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	var lines []string
	code := Run(root, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "NOT checked") {
		t.Fatalf("missing tool must be said: %v", lines)
	}
	if code == 0 {
		t.Fatalf("a check that did not run must not exit 0:\n%s", joined)
	}
	if !strings.Contains(joined, "did NOT run") {
		t.Errorf("the summary must name how many checks were skipped:\n%s", joined)
	}
}

// The thresholds are repo-overridable (D-OVERRIDE): the generated isolated
// config carries the repo's numbers, defaults filling the gaps.
func TestThresholdsComeFromRepoConfig(t *testing.T) {
	cfg := config.Config{Gocyclo: 25, FunlenLines: 120}
	got := golangciCfg(cfg)
	for _, want := range []string{"min-complexity: 25", "lines: 120", "statements: 50"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in generated config:\n%s", want, got)
		}
	}
}

// proved by: returning `found` instead of `true` in hasFiles' walk-error branch
//
// hasFiles gates whether an ecosystem's complexity check runs at all. A
// failed survey answering "no files of this type" silently skips that
// ecosystem, which reads exactly like a clean result. When the survey
// could not look, the honest answer is to let the tool run and report for
// itself — it says NOT checked when it cannot.
func TestUnwalkableTreeErrsTowardRunningTheCheck(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod does not deny the walk here")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(root, 0o755); err != nil {
			t.Error(err)
		}
	})

	if !hasFiles(root, ".go") {
		t.Fatal("a survey that could not look must not answer \"no files of this type\"")
	}
}

// A long function is usually also a branchy one, so funlen and gocyclo
// land on the same line — and golangci-lint's uniq-by-line processor is
// ON by default, keeping only the first issue per line. Every funlen
// finding in this repository was being dropped that way: 31 complexity
// lines, 0 length lines, over a dispatch function 343 lines long.
//
// proved by: removed `uniq-by-line: false` from the generated config —
// the funlen finding disappears and only gocyclo is reported.
func TestLengthAndComplexityOnTheSameLineAreBothReported(t *testing.T) {
	if tools.Resolve(lint.GolangciLint, "") == "" {
		t.Skip("golangci-lint not installed")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n\ngo 1.22\n"), 0o644)

	// one function that is both too branchy and too long, so both linters
	// report it at the same file:line
	var b strings.Builder
	b.WriteString("package demo\n\nfunc Sprawling(x int) int {\n\tsum := 0\n\tswitch {\n")
	for i := 0; i < 20; i++ {
		b.WriteString("\tcase x == " + strings.Repeat("1", i+1) + ":\n\t\tsum++\n\t\tsum += x\n\t\tsum -= 1\n\t\tsum *= 2\n")
	}
	b.WriteString("\t}\n\treturn sum\n}\n")
	os.WriteFile(filepath.Join(root, "demo.go"), []byte(b.String()), 0o644)

	var lines []string
	Run(root, func(s string) { lines = append(lines, s) })
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "(gocyclo)") {
		t.Fatalf("the fixture must trip gocyclo, or the test proves nothing:\n%s", joined)
	}
	if !strings.Contains(joined, "(funlen)") {
		t.Errorf("the same function is too long, and that finding is being dropped:\n%s", joined)
	}
}
