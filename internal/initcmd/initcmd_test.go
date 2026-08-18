package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A repo whose only source file is Go, so gofmt is the one required tool.
func goRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeBin creates a directory holding executable stubs with the given names,
// so PATH can be pointed at exactly the managers a scenario wants to exist.
func fakeBin(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestInstalledToolProducesNoStep(t *testing.T) {
	dir := goRepo(t)
	// gofmt genuinely present (via the real PATH) means no plan.
	if len(Plan(dir)) != 0 {
		// If this machine has no gofmt the premise fails; skip rather than lie.
		t.Skip("gofmt not on PATH here")
	}
	var out bytes.Buffer
	if code := Run(dir, false, &out); code != 0 {
		t.Fatalf("exit %d with nothing missing, want 0\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "every formatter") {
		t.Fatalf("did not say it was complete:\n%s", out.String())
	}
}

func TestMissingToolPicksTheAvailableManager(t *testing.T) {
	skipOnWindows(t)
	dir := goRepo(t)
	// Only brew exists; gofmt does not.
	t.Setenv("PATH", fakeBin(t, "brew"))
	steps := Plan(dir)
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(steps))
	}
	if steps[0].Manager != "brew" {
		t.Fatalf("chose %q, want brew", steps[0].Manager)
	}
}

func TestPreferenceOrderIsTheListOrder(t *testing.T) {
	skipOnWindows(t)
	dir := goRepo(t)
	// brew and apt-get both exist; brew is listed first for gofmt.
	t.Setenv("PATH", fakeBin(t, "brew", "apt-get"))
	steps := Plan(dir)
	if len(steps) != 1 || steps[0].Manager != "brew" {
		t.Fatalf("steps = %+v, want one step via brew", steps)
	}
}

func TestNoManagerFallsBackToTheHumanLine(t *testing.T) {
	skipOnWindows(t)
	dir := goRepo(t)
	t.Setenv("PATH", fakeBin(t)) // nothing at all
	steps := Plan(dir)
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(steps))
	}
	if steps[0].Manager != "" || steps[0].Fallback == "" {
		t.Fatalf("step = %+v, want fallback-only", steps[0])
	}
	var out bytes.Buffer
	if code := Run(dir, false, &out); code != 1 {
		t.Fatalf("exit %d with a gap, want 1", code)
	}
	if !strings.Contains(out.String(), "install by hand") {
		t.Fatalf("fallback not printed:\n%s", out.String())
	}
}

func TestPlanOnlyExitsNonZeroWhileGapsRemain(t *testing.T) {
	skipOnWindows(t)
	dir := goRepo(t)
	t.Setenv("PATH", fakeBin(t, "brew"))
	var out bytes.Buffer
	if code := Run(dir, false, &out); code != 1 {
		t.Fatalf("exit %d, want 1 — a printed plan leaves the gap open", code)
	}
	if !strings.Contains(out.String(), "brew install go") {
		t.Fatalf("plan does not show the command:\n%s", out.String())
	}
}

// --yes runs the chosen command and then re-surveys. The fake brew records its
// argv and "installs" nothing, so the re-survey must still find the gap and
// the exit must say so — an installer's exit 0 is a claim, not the fact.
func TestYesExecutesAndTrustsTheResurveyNotTheInstaller(t *testing.T) {
	skipOnWindows(t)
	dir := goRepo(t)
	bin := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "argv.log")
	script := "#!/bin/sh\necho \"$@\" >> " + logFile + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "brew"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	var out bytes.Buffer
	code := Run(dir, true, &out)

	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("the installer was never executed: %v\n%s", err, out.String())
	}
	if !strings.Contains(string(logged), "install go") {
		t.Fatalf("installer argv = %q, want 'install go'", string(logged))
	}
	if code == 0 {
		t.Fatal("exit 0 although the tool still does not answer — trusted the installer over the re-survey")
	}
	if !strings.Contains(out.String(), "still missing") {
		t.Fatalf("did not report the remaining gap:\n%s", out.String())
	}
}

// Windows cannot execute the #!/bin/sh stubs these tests build their fake
// tools from; the POSIX legs of the CI matrix carry this coverage.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sh-stub fixtures cannot run on Windows")
	}
}
