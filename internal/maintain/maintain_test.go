package maintain

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestMissingToolsSayNotCheckedAndStillExitZero(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module demo\n"), 0o644)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	var lines []string
	if code := Run(root, func(s string) { lines = append(lines, s) }); code != 0 {
		t.Fatalf("report-only command must exit 0: %v", lines)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "NOT checked") {
		t.Fatalf("missing tool must be said: %v", lines)
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
