package testrun

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func collect() (func(string), *[]string) {
	var lines []string
	return func(s string) { lines = append(lines, s) }, &lines
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
}

const passingTest = `package demo

import "testing"

func TestAdd(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("math broke")
	}
}
`

const failingTest = `package broken

import "testing"

func TestAlwaysRed(t *testing.T) {
	t.Fatal("deliberately red")
}
`

func goFixture(t *testing.T, withFailure bool) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	write(t, root, "demo.go", "package demo\n\nfunc Add(a, b int) int { return a + b }\n")
	write(t, root, "demo_test.go", passingTest)
	if withFailure {
		write(t, root, "broken/broken.go", "package broken\n")
		write(t, root, "broken/broken_test.go", failingTest)
	}
	return root
}

func TestGoFailThenPass(t *testing.T) {
	requireGo(t)
	root := goFixture(t, true)
	out, lines := collect()
	if code := Report(Run(root, nil, false, ""), out); code != 1 {
		t.Fatalf("a failing package must exit 1, got %d:\n%s", code, strings.Join(*lines, "\n"))
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "TestAlwaysRed") {
		t.Fatalf("the failing test must be named: %v", *lines)
	}

	// fix the failure; the same command must go green
	write(t, root, "broken/broken_test.go", strings.Replace(failingTest, "t.Fatal(\"deliberately red\")", "_ = t", 1))
	out2, lines2 := collect()
	if code := Report(Run(root, nil, false, ""), out2); code != 0 {
		t.Fatalf("after the fix the suite must pass, got %d:\n%s", code, strings.Join(*lines2, "\n"))
	}
}

func TestNoSetupIsNotRun(t *testing.T) {
	out, lines := collect()
	if code := Report(Run(t.TempDir(), nil, false, ""), out); code != 2 {
		t.Fatalf("no test setup must exit 2, got %d: %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "NOT run") {
		t.Fatalf("the honesty rule wants NOT run, got: %v", *lines)
	}
}

func TestGoCoverageIsReported(t *testing.T) {
	requireGo(t)
	root := goFixture(t, false)
	results := Run(root, nil, true, "")
	if len(results) != 1 || results[0].Coverage < 0 {
		t.Fatalf("go coverage must be measured: %+v", results)
	}
}

func TestJSWithoutTestScriptIsNotRunNotFail(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name":"x","scripts":{"test":"echo \"Error: no test specified\" && exit 1"}}`)
	results := Run(root, nil, false, "")
	if len(results) != 1 || results[0].Verdict != NotRun {
		t.Fatalf("the npm placeholder is no test script — must be NOT run: %+v", results)
	}
}

func TestDualEcosystemReportsSeparately(t *testing.T) {
	requireGo(t)
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("no npm on PATH")
	}
	root := goFixture(t, false)
	write(t, root, "package.json", `{"name":"x","scripts":{"test":"node -e \"process.exit(0)\""}}`)
	write(t, root, "package-lock.json", "{}")
	results := Run(root, nil, false, "")
	if len(results) != 2 {
		t.Fatalf("both ecosystems must run: %+v", results)
	}
	for _, r := range results {
		if r.Verdict != Pass {
			t.Fatalf("%s must pass: %+v", r.Ecosystem, r)
		}
	}
}

func TestSuiteClosureVerdicts(t *testing.T) {
	requireGo(t)
	ok, summary := Suite(goFixture(t, true))()
	if ok || !strings.Contains(summary, "go:") {
		t.Fatalf("a red suite must not be ok: %v %q", ok, summary)
	}
	ok, _ = Suite(goFixture(t, false))()
	if !ok {
		t.Fatal("a green suite must be ok")
	}
	ok, summary = Suite(t.TempDir())()
	if ok || !strings.Contains(summary, "NOT be verified") {
		t.Fatalf("no setup must block under the policy: %v %q", ok, summary)
	}
}

func TestGoTargetsNarrowing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pkg/a/a.go", "package a\n")
	got := goTargets(root, []string{filepath.Join(root, "pkg", "a", "a.go"), "pkg/a"})
	if len(got) != 1 || got[0] != "./pkg/a" {
		t.Fatalf("paths must dedupe to package patterns: %v", got)
	}
	if got := goTargets(root, nil); got[0] != "./..." {
		t.Fatalf("no paths means the whole module: %v", got)
	}
}
