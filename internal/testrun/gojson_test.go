package testrun

import (
	"fmt"
	"strings"
	"testing"
)

// goModule writes a module with the given test files and runs the Go
// runner over it, the way `procoder test` would.
func goModule(t *testing.T, files map[string]string) Result {
	t.Helper()
	requireGo(t)
	root := t.TempDir()
	write(t, root, "go.mod", "module shapes\n\ngo 1.22\n")
	for name, body := range files {
		write(t, root, name, body)
	}
	return runGo(root, nil, false, "")
}

// proved by: restoring the `^--- FAIL: (\S+)` scrape over the text stream
// — a test that PRINTS a framework-looking line is then reported as a
// failing test it never ran, which is hypothesis (1) of #283 shown real.
func TestAPrintedFailLineIsNotAFailingTest(t *testing.T) {
	r := goModule(t, map[string]string{
		"spoof/s_test.go": "package spoof\nimport (\"fmt\"; \"testing\")\n" +
			"func TestLoud(t *testing.T) { fmt.Println(\"--- FAIL: TestNoDirectProcoderFileIO (0.10s)\") }\n" +
			"func TestReal(t *testing.T) { t.Error(\"the real one\") }\n",
	})
	if r.Verdict != Fail {
		t.Fatalf("a red package must fail: %+v", r)
	}
	if strings.Contains(r.Detail, "TestNoDirectProcoderFileIO") || r.Failed != 1 {
		t.Fatalf("only the framework's verdict names a failing test; got %d: %s", r.Failed, r.Detail)
	}
	if !strings.Contains(r.Detail, "TestReal") {
		t.Fatalf("the real failure must be named: %s", r.Detail)
	}
}

// proved by: dropping packageOnlyFailures from failDetail — a package that
// crashed outside any test disappears from a report that names a test
// failure elsewhere, and the gate says one thing is red when two are.
func TestAPackageThatFailedOutsideAnyTestIsNamed(t *testing.T) {
	r := goModule(t, map[string]string{
		"red/r_test.go": "package red\nimport \"testing\"\nfunc TestRed(t *testing.T) { t.Error(\"red\") }\n",
		"gor/g_test.go": "package gor\nimport (\"testing\"; \"time\")\n" +
			"func TestInnocent(t *testing.T) { go func() { panic(\"background boom\") }(); time.Sleep(time.Second) }\n",
		"broken/b_test.go": "package broken\nimport \"testing\"\nfunc TestX(t *testing.T) { undefined() }\n",
	})
	for _, want := range []string{"TestRed", "shapes/gor", "shapes/broken"} {
		if !strings.Contains(r.Detail, want) {
			t.Errorf("detail must name %s: %s", want, r.Detail)
		}
	}
	if strings.Contains(r.Detail, "TestInnocent") {
		t.Errorf("the test that happened to be running when the process died did not fail: %s", r.Detail)
	}
	if !strings.Contains(r.Output, "background boom") {
		t.Errorf("the excerpt must carry the crash:\n%s", r.Output)
	}
	if !strings.Contains(r.Output, "undefined") {
		t.Errorf("the excerpt must carry the build error:\n%s", r.Output)
	}
}

// proved by: emptying packageOnlyFailures' result in failDetail — the
// detail falls back to the first line of whatever go printed, which is
// the old behaviour, and says neither how many packages failed nor that
// no test did.
func TestALoneBuildFailureIsNamedByPackage(t *testing.T) {
	r := goModule(t, map[string]string{
		"aok/a_test.go":    "package aok\nimport \"testing\"\nfunc TestFine(t *testing.T) {}\n",
		"broken/b_test.go": "package broken\nimport \"testing\"\nfunc TestX(t *testing.T) { undefined() }\n",
	})
	if r.Verdict != Fail || r.Detail != "1 package(s) failed outside any test: shapes/broken [build failed]" {
		t.Fatalf("a build failure must be named by package and reason: %+v", r)
	}
}

// proved by: counting every fail event instead of leafFailures — a failed
// subtest is then reported twice, once under its parent's name.
func TestAFailedSubtestIsNamedOnce(t *testing.T) {
	r := goModule(t, map[string]string{
		"sub/s_test.go": "package sub\nimport \"testing\"\n" +
			"func TestParent(t *testing.T) { t.Run(\"child\", func(t *testing.T) { t.Fatal(\"child broke\") }) }\n",
	})
	if r.Failed != 1 || !strings.Contains(r.Detail, "TestParent/child") {
		t.Fatalf("one failure, named by the subtest that failed: %d %s", r.Failed, r.Detail)
	}
	if !strings.Contains(r.Output, "child broke") {
		t.Fatalf("the excerpt must carry the assertion:\n%s", r.Output)
	}
}

// proved by: attributing output by position in the stream rather than by
// the event's Test field — parallel tests interleave, and the passing
// test's log then lands in the failing test's excerpt.
func TestParallelOutputStaysWithItsTest(t *testing.T) {
	r := goModule(t, map[string]string{
		"par/p_test.go": "package par\nimport (\"testing\"; \"time\")\n" +
			"func TestA(t *testing.T) { t.Parallel(); time.Sleep(50 * time.Millisecond); t.Error(\"A red\") }\n" +
			"func TestB(t *testing.T) { t.Parallel(); t.Log(\"B was fine\") }\n",
		"pan/p_test.go": "package pan\nimport \"testing\"\n" +
			"func TestPanics(t *testing.T) { var m map[string]int; m[\"x\"] = 1 }\n",
	})
	if r.Failed != 2 || !strings.Contains(r.Detail, "TestA") || !strings.Contains(r.Detail, "TestPanics") {
		t.Fatalf("both failures named: %d %s", r.Failed, r.Detail)
	}
	if strings.Contains(r.Output, "B was fine") {
		t.Fatalf("a passing test's output is not a failure's:\n%s", r.Output)
	}
	for _, want := range []string{"A red", "assignment to entry in nil map"} {
		if !strings.Contains(r.Output, want) {
			t.Errorf("excerpt missing %q:\n%s", want, r.Output)
		}
	}
}

// proved by: dropping the excerpt from findingFor — the gate then reports
// a failing test by name alone, which is exactly the undiagnosable report
// #283 was filed from.
func TestTheGateFindingCarriesTheAssertion(t *testing.T) {
	r := goModule(t, map[string]string{
		"red/r_test.go": "package red\nimport \"testing\"\nfunc TestRed(t *testing.T) { t.Errorf(\"want %d, got %d\", 1, 2) }\n",
	})
	f := findingFor(r, true)
	if f == nil {
		t.Fatal("a failing suite is a finding")
	}
	first, rest, _ := strings.Cut(f.Message, "\n")
	if !strings.Contains(first, "TestRed") {
		t.Fatalf("the first line still names the test: %q", first)
	}
	if !strings.Contains(rest, "want 1, got 2") || !strings.HasPrefix(rest, "        ") {
		t.Fatalf("the assertion follows, indented under the finding:\n%s", f.Message)
	}

	out, lines := collect()
	Report([]Result{r}, out)
	if !strings.Contains(strings.Join(*lines, "\n"), "want 1, got 2") {
		t.Fatalf("procoder test prints the excerpt too: %v", *lines)
	}
}

// proved by: removing bound() or the total cap from excerpt — a test that
// prints thousands of lines then pours all of them into the gate.
func TestTheExcerptIsBounded(t *testing.T) {
	var b strings.Builder
	for i := range 5 {
		name := fmt.Sprintf("TestLoud%d", i)
		fmt.Fprintf(&b, `{"Action":"run","Package":"p","Test":%q}`+"\n", name)
		for j := range 1000 {
			fmt.Fprintf(&b, `{"Action":"output","Package":"p","Test":%q,"Output":"line %d %s\n"}`+"\n", name, j, strings.Repeat("x", 500))
		}
		fmt.Fprintf(&b, `{"Action":"fail","Package":"p","Test":%q}`+"\n", name)
	}
	b.WriteString(`{"Action":"fail","Package":"p"}` + "\n")
	ex := parseGoJSON(b.String()).excerpt()
	if len(ex) > excerptTotalMax+64 {
		t.Fatalf("excerpt is %d bytes, cap is %d", len(ex), excerptTotalMax)
	}
	for _, l := range strings.Split(ex, "\n") {
		if len(l) > excerptLineMax+16 {
			t.Fatalf("a line of %d bytes escaped the line cap", len(l))
		}
	}
	if !strings.Contains(ex, "line 0 ") {
		t.Fatalf("the head of a failure is kept — it is where a panic names itself:\n%s", ex)
	}
	if !strings.Contains(ex, "elided") {
		t.Fatalf("a cut says so:\n%s", ex)
	}
}

// proved by: dropping the stray-line fallback — before Go 1.24 a build
// error is stderr text outside the JSON stream, and the excerpt for the
// failed package would then be empty of the one line that says what broke.
func TestPreJSONBuildErrorsStillReachTheExcerpt(t *testing.T) {
	raw := "# shapes/broken [shapes/broken.test]\n" +
		"broken/b_test.go:3:27: undefined: undefined\n" +
		`{"Action":"start","Package":"shapes/broken"}` + "\n" +
		`{"Action":"output","Package":"shapes/broken","Output":"FAIL\tshapes/broken [build failed]\n"}` + "\n" +
		`{"Action":"fail","Package":"shapes/broken"}` + "\n"
	run := parseGoJSON(raw)
	detail, _ := run.failDetail()
	if !strings.Contains(detail, "shapes/broken [build failed]") {
		t.Fatalf("detail: %s", detail)
	}
	if !strings.Contains(run.excerpt(), "undefined: undefined") {
		t.Fatalf("excerpt:\n%s", run.excerpt())
	}
}
