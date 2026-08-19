package testrun

import (
	"strings"
	"testing"
)

// twoPackageFixture is a module with one test function per package, so a
// --name filter has something real to narrow: TestAlpha in ./alpha,
// TestBeta in ./beta.
func twoPackageFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module demo\n\ngo 1.22\n")
	write(t, root, "alpha/alpha.go", "package alpha\n")
	write(t, root, "alpha/alpha_test.go", "package alpha\n\nimport \"testing\"\n\nfunc TestAlpha(t *testing.T) {}\n")
	write(t, root, "beta/beta.go", "package beta\n")
	write(t, root, "beta/beta_test.go", "package beta\n\nimport \"testing\"\n\nfunc TestBeta(t *testing.T) {}\n")
	return root
}

func TestGoArgsCarryFilterPathsAndCoverage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pkg/a/a.go", "package a\n")
	got := strings.Join(goArgs(root, []string{"pkg/a"}, true, "TestAdd"), " ")
	if got != "test -cover -run=TestAdd ./pkg/a" {
		t.Fatalf("--name must compose with --coverage and paths: %q", got)
	}
	// the joined -run= form is the point: a dash-leading pattern must stay a
	// pattern, never become a flag
	got = strings.Join(goArgs(root, nil, false, "-weird"), " ")
	if got != "test -run=-weird ./..." {
		t.Fatalf("go must express any pattern: %q", got)
	}
	if got := strings.Join(goArgs(root, nil, false, ""), " "); got != "test ./..." {
		t.Fatalf("no name means today's argv: %q", got)
	}
}

func TestPytestArgsCarryFilterPathsAndCoverage(t *testing.T) {
	args, ok := pytestArgs([]string{"tests/test_x.py"}, true, "adds")
	if !ok || strings.Join(args, " ") != "-q --cov=. -k adds tests/test_x.py" {
		t.Fatalf("pytest -k must compose with --cov and paths: %v %v", args, ok)
	}
	args, ok = pytestArgs(nil, false, "-k")
	if ok || strings.Join(args, " ") != "-q" {
		t.Fatalf(`a dash-leading pattern is not expressible for pytest: %v %v`, args, ok)
	}
}

func TestCargoJSGradleMavenFilterArgs(t *testing.T) {
	if args, ok := cargoArgs("adds"); !ok || strings.Join(args, " ") != "test --quiet adds" {
		t.Fatalf("cargo takes the pattern positionally: %v %v", args, ok)
	}
	if args, ok := cargoArgs("-x"); ok || strings.Join(args, " ") != "test --quiet" {
		t.Fatalf("cargo cannot express a dash-leading pattern: %v %v", args, ok)
	}
	// the JS pattern goes after `--`, as ONE argv element, untranslated
	got := jsArgs("renders a list")
	if len(got) != 3 || got[0] != "test" || got[1] != "--" || got[2] != "renders a list" {
		t.Fatalf("the JS pattern must be one argv element after `--`: %q", got)
	}
	if got := jsArgs(""); len(got) != 1 || got[0] != "test" {
		t.Fatalf("no name means a bare test script: %q", got)
	}
	if args, ok := gradleArgs("com.x.FooTest"); !ok || strings.Join(args, " ") != "test -q --tests com.x.FooTest" {
		t.Fatalf("gradle uses --tests: %v %v", args, ok)
	}
	if args, ok := gradleArgs("-Pfoo"); ok || strings.Join(args, " ") != "test -q" {
		t.Fatalf("gradle cannot express a dash-leading pattern: %v %v", args, ok)
	}
	// maven's -Dtest= is joined, so even a dash-leading pattern arrives whole
	if args, ok := mavenArgs("-Weird"); !ok || strings.Join(args, " ") != "-q test -Dtest=-Weird" {
		t.Fatalf("maven joins the property: %v %v", args, ok)
	}
	if args, ok := mavenArgs(""); ok || strings.Join(args, " ") != "-q test" {
		t.Fatalf("no name means no filter and Filtered false: %v %v", args, ok)
	}
}

func TestUnfilterableRunnerSaysNotFiltered(t *testing.T) {
	got := filterNote("pass (3 test(s))", "-x", false, "cargo cannot")
	if !strings.Contains(got, "NOT filtered") || !strings.Contains(got, "the whole suite ran") {
		t.Fatalf("an unfilterable runner must say so: %q", got)
	}
	if got := filterNote("pass", "adds", true, "unused"); !strings.Contains(got, `filtered to "adds"`) || strings.Contains(got, "NOT filtered") {
		t.Fatalf("a filtered run must say what it filtered to: %q", got)
	}
	if got := filterNote("pass", "", false, "unused"); got != "pass" {
		t.Fatalf("no --name means no label at all: %q", got)
	}
}

func TestGoNameRunsOnlyTheMatchingTest(t *testing.T) {
	requireGo(t)
	root := twoPackageFixture(t)
	results := Run(root, nil, false, "TestAlpha")
	if len(results) != 1 || results[0].Verdict != Pass || !results[0].Filtered {
		t.Fatalf("a filtered go run must pass and be marked filtered: %+v", results)
	}
	if !strings.Contains(results[0].Detail, "1 package(s) matched") {
		t.Fatalf("only the matching package must have run: %q", results[0].Detail)
	}
	out, lines := collect()
	if code := Report(results, out); code != 0 {
		t.Fatalf("a filtered green run exits 0, got %d: %v", code, *lines)
	}
}

func TestGoNameMatchingNothingIsAnHonestPass(t *testing.T) {
	requireGo(t)
	results := Run(twoPackageFixture(t), nil, false, "TestNoSuchThing")
	if len(results) != 1 || results[0].Verdict != Pass {
		t.Fatalf("zero matches is a pass, not a failure: %+v", results)
	}
	if !strings.Contains(results[0].Detail, "0 test(s) matched") {
		t.Fatalf("a silent green would be a lie: %q", results[0].Detail)
	}
}

func TestNotRunIsNeverFiltered(t *testing.T) {
	// NOT run and NOT filtered must never collapse into each other: a
	// missing runner filtered nothing at all.
	r := notRun(Result{Ecosystem: "rust", Filtered: true}, "cargo is not installed")
	if r.Filtered || !strings.Contains(r.Detail, "NOT run") {
		t.Fatalf("a runner that never ran filtered nothing: %+v", r)
	}
}
