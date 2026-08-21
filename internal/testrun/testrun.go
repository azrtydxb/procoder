// Package testrun closes the oldest hole in "done": it runs the
// repository's actual test suite. One command detects every ecosystem's
// canonical runner, runs it, and answers honestly — PASS with counts where
// the output allows, FAIL with the failing lines, and NOT run when a
// runner is missing or absent, which is never the same as green. Under
// `[test] policy = "block"` the close controllers consume the verdict:
// a failing or unverifiable suite keeps work open (unknown is never done).
package testrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"procoder/internal/textutil"
)

const runTimeout = 10 * time.Minute

// Verdict values for one ecosystem's run.
const (
	Pass   = "pass"
	Fail   = "fail"
	NotRun = "notrun"
)

// Result is one ecosystem's answer.
type Result struct {
	Ecosystem string
	Verdict   string
	Detail    string // the honest one-liner: counts, reason, or excerpt
	Passed    int    // parsed where the runner's output allows, else 0
	Failed    int
	Coverage  float64 // percent; <0 means not measured
	// NoSuite distinguishes "there is nothing to run here" from "the run
	// could not happen". Both print as NOT run; only the second is a check
	// that failed to answer.
	NoSuite bool
	// Filtered records whether a --name pattern actually reached this
	// runner. False with a non-empty name means the runner ran the WHOLE
	// suite: the Detail says "NOT filtered" and never wears a filtered
	// label over an unfiltered run.
	Filtered bool
}

// Run executes every detected runner. paths narrow the Go package list and
// the pytest targets; other runners keep their native whole-project
// granularity (their Detail says so when paths were given). name, when
// non-empty, narrows the run to matching tests in each runner's OWN syntax
// — untranslated between ecosystems, and reported as NOT filtered wherever
// the runner cannot express it.
func Run(root string, paths []string, coverage bool, name string) []Result {
	var out []Result
	if exists(root, "go.mod") {
		out = append(out, runGo(root, paths, coverage, name))
	}
	if exists(root, "Cargo.toml") {
		out = append(out, runCargo(root, name))
	}
	if exists(root, "package.json") {
		out = append(out, runJS(root, name))
	}
	if pytestDetected(root) {
		out = append(out, runPytest(root, paths, coverage, name))
	}
	if exists(root, "build.gradle") || exists(root, "build.gradle.kts") || exists(root, "pom.xml") {
		out = append(out, runJava(root, name))
	}
	if phpunitDetected(root) {
		out = append(out, runPHPUnit(root, coverage, name))
	}
	return out
}

// Report prints results and returns the command's exit code: 1 when any
// suite failed, 0 when at least one passed and none failed, 2 when nothing
// could run at all.
func Report(results []Result, out func(string)) int {
	if len(results) == 0 {
		out("NOT run — no recognized test setup in this repository (go.mod, Cargo.toml, package.json test script, pytest, gradle/maven, phpunit)")
		return 2
	}
	failed, ran := false, false
	for _, r := range results {
		mark := "  ok  "
		switch r.Verdict {
		case Fail:
			mark = "  FAIL"
			failed = true
			ran = true
		case NotRun:
			mark = "  ----"
		default:
			ran = true
		}
		line := fmt.Sprintf("%s %-8s %s", mark, r.Ecosystem, r.Detail)
		if r.Coverage >= 0 {
			line += fmt.Sprintf("  coverage %.1f%%", r.Coverage)
		}
		out(line)
	}
	switch {
	case failed:
		return 1
	case !ran:
		return 2
	default:
		return 0
	}
}

// Suite is the closure the close controllers call under the block policy:
// ok only when at least one suite ran and none failed — a suite that could
// not be verified blocks exactly like a failing one.
func Suite(root string) func() (bool, string) {
	return func() (bool, string) {
		results := Run(root, nil, false, "")
		if len(results) == 0 {
			return false, "no recognized test setup — the suite could NOT be verified"
		}
		var parts []string
		ok, ran := true, false
		for _, r := range results {
			parts = append(parts, r.Ecosystem+": "+r.Detail)
			switch r.Verdict {
			case Fail:
				ok = false
			case Pass:
				ran = true
			}
		}
		return ok && ran, strings.Join(parts, " · ")
	}
}

var (
	goFailRe   = regexp.MustCompile(`(?m)^--- FAIL: (\S+)`)
	goOkPkgRe  = regexp.MustCompile(`(?m)^ok\s`)
	goCoverRe  = regexp.MustCompile(`coverage:\s+([0-9.]+)% of statements`)
	cargoSumRe = regexp.MustCompile(`test result: (\w+)\. (\d+) passed; (\d+) failed`)
	pytestRe   = regexp.MustCompile(`(?:(\d+) failed)?(?:, )?(\d+) passed`)
	pytestCov  = regexp.MustCompile(`(?m)^TOTAL\s+\d+\s+\d+\s+(\d+)%`)
)

// dashPattern reports whether a pattern begins with a dash. Runners that
// take their filter as the NEXT argv element (pytest -k, cargo's positional
// TESTNAME, gradle --tests) would read such a pattern as one of their own
// flags: there is no way to express it, so we say NOT filtered instead of
// mangling the argv or pretending.
func dashPattern(name string) bool { return strings.HasPrefix(name, "-") }

// filtered appends the honest filter label to a Detail: what the filter was
// when it reached the runner, or why it did not and that the whole suite
// ran regardless. why is only consulted when ok is false.
func filterNote(detail, name string, ok bool, why string) string {
	switch {
	case name == "":
		return detail
	case ok:
		return detail + fmt.Sprintf(" — filtered to %q", name)
	default:
		return detail + fmt.Sprintf(" — NOT filtered (%s); the whole suite ran", why)
	}
}

// goArgs builds `go test`'s argv. -run= uses the joined form on purpose: a
// pattern starting with a dash must never be read as a flag, so Go can
// always express the filter.
func goArgs(root string, paths []string, coverage bool, name string) []string {
	args := []string{"test"}
	if coverage {
		args = append(args, "-cover")
	}
	if name != "" {
		args = append(args, "-run="+name)
	}
	return append(args, goTargets(root, paths)...)
}

func runGo(root string, paths []string, coverage bool, name string) Result {
	r := Result{Ecosystem: "go", Coverage: -1, Filtered: name != ""}
	bin, err := exec.LookPath("go")
	if err != nil {
		return notRun(r, "the go toolchain is not installed")
	}
	args := goArgs(root, paths, coverage, name)
	raw, err, timedOut := execute(root, bin, args)
	if timedOut {
		return notRun(r, "go test gave no answer in "+runTimeout.String())
	}
	if err != nil {
		fails := goFailRe.FindAllStringSubmatch(raw, -1)
		names := make([]string, 0, len(fails))
		for _, f := range fails {
			names = append(names, f[1])
		}
		detail := "FAILED"
		if len(names) > 0 {
			detail = fmt.Sprintf("%d test(s) failing: %s", len(names), strings.Join(cap3(names), ", "))
		} else {
			detail = "FAILED — " + textutil.FirstLine(raw+errStr(err))
		}
		r.Verdict, r.Detail, r.Failed = Fail, filterNote(detail, name, true, ""), len(names)
		return r
	}
	pkgs := len(goOkPkgRe.FindAllString(raw, -1))
	r.Verdict = Pass
	r.Detail = fmt.Sprintf("pass (%d package(s))", pkgs)
	if strings.Contains(raw, "[no test files]") && pkgs == 0 {
		r.Detail = "pass — but no test files exist yet"
	}
	if name != "" {
		// go test exits 0 when -run matches nothing, marking each package
		// "[no tests to run]". A bare green there would imply the suite ran,
		// so say how many packages the pattern actually reached.
		matched := pkgs - strings.Count(raw, "[no tests to run]")
		if matched <= 0 {
			r.Detail = fmt.Sprintf("pass — 0 test(s) matched %q", name)
			return r
		}
		r.Detail = fmt.Sprintf("pass (%d package(s) matched %q)", matched, name)
	}
	if coverage {
		if covs := goCoverRe.FindAllStringSubmatch(raw, -1); len(covs) > 0 {
			sum := 0.0
			for _, c := range covs {
				v, _ := strconv.ParseFloat(c[1], 64)
				sum += v
			}
			r.Coverage = sum / float64(len(covs))
			r.Detail += fmt.Sprintf(" — mean of %d covered package(s)", len(covs))
		}
	}
	return r
}

// goTargets maps narrowing paths onto package patterns; no paths means the
// whole module.
func goTargets(root string, paths []string) []string {
	if len(paths) == 0 {
		return []string{"./..."}
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, p)
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			rel = filepath.Dir(rel)
		}
		pat := "./" + filepath.ToSlash(rel)
		if !seen[pat] {
			seen[pat] = true
			out = append(out, pat)
		}
	}
	if len(out) == 0 {
		return []string{"./..."}
	}
	return out
}

// cargoArgs builds `cargo test`'s argv. The filter is cargo's positional
// TESTNAME, so a dash-leading pattern cannot be expressed: ok is false and
// the whole suite runs, said out loud.
func cargoArgs(name string) (args []string, ok bool) {
	args = []string{"test", "--quiet"}
	if name == "" || dashPattern(name) {
		return args, false
	}
	return append(args, name), true
}

func runCargo(root string, name string) Result {
	r := Result{Ecosystem: "rust", Coverage: -1}
	bin, err := exec.LookPath("cargo")
	if err != nil {
		return notRun(r, "cargo is not installed")
	}
	args, ok := cargoArgs(name)
	r.Filtered = ok
	why := `cargo takes the filter as a positional TESTNAME, so a pattern beginning with "-" would be read as a flag`
	raw, err, timedOut := execute(root, bin, args)
	if timedOut {
		return notRun(r, "cargo test gave no answer in "+runTimeout.String())
	}
	if m := cargoSumRe.FindStringSubmatch(raw); m != nil {
		r.Passed, _ = strconv.Atoi(m[2])
		r.Failed, _ = strconv.Atoi(m[3])
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = filterNote(fmt.Sprintf("FAILED (%d passed, %d failed) — %s", r.Passed, r.Failed, textutil.FirstLine(raw+errStr(err))), name, ok, why)
		return r
	}
	r.Verdict = Pass
	detail := fmt.Sprintf("pass (%d test(s))", r.Passed)
	if ok && r.Passed == 0 {
		detail = fmt.Sprintf("pass — 0 test(s) matched %q", name)
		r.Detail = detail
		return r
	}
	r.Detail = filterNote(detail, name, ok, why)
	return r
}

// jsArgs builds the package manager's argv. jest and vitest disagree about
// what -t means, so procoder translates nothing: the pattern goes after the
// `--` separator, untouched, as one argv element, and the Detail says the
// filtering is the runner's own.
func jsArgs(name string) []string {
	args := []string{"test"}
	if name == "" {
		return args
	}
	return append(args, "--", name)
}

func runJS(root string, name string) Result {
	r := Result{Ecosystem: "js", Coverage: -1}
	script := testScript(root)
	if script == "" {
		return noSuite(r, "package.json has no test script")
	}
	mgr := "npm"
	switch {
	case exists(root, "bun.lockb") || exists(root, "bun.lock"):
		mgr = "bun"
	case exists(root, "pnpm-lock.yaml"):
		mgr = "pnpm"
	case exists(root, "yarn.lock"):
		mgr = "yarn"
	}
	bin, err := exec.LookPath(mgr)
	if err != nil {
		return notRun(r, mgr+" is not installed")
	}
	r.Filtered = name != ""
	raw, err, timedOut := execute(root, bin, jsArgs(name))
	if timedOut {
		return notRun(r, mgr+" test gave no answer in "+runTimeout.String())
	}
	delegated := ""
	if name != "" {
		delegated = fmt.Sprintf(" — pattern %q passed after `--`; filtering is delegated to the test runner", name)
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = "FAILED — " + firstFailingLine(raw, errStr(err)) + delegated
		return r
	}
	r.Verdict = Pass
	r.Detail = "pass (" + mgr + " test)" + delegated
	return r
}

// testScript returns package.json's test script, empty when absent or the
// npm placeholder that only echoes an error.
func testScript(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(raw, &pkg) != nil {
		return ""
	}
	s := pkg.Scripts["test"]
	if strings.Contains(s, "no test specified") {
		return ""
	}
	return s
}

func pytestDetected(root string) bool {
	if exists(root, "pytest.ini") {
		return true
	}
	if raw, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil &&
		strings.Contains(string(raw), "[tool.pytest") {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(root, "tests", "test_*.py"))
	return len(matches) > 0
}

// pytestArgs builds pytest's argv. -k takes its expression as the NEXT argv
// element, so a dash-leading pattern cannot be expressed: ok is false and
// the caller reports NOT filtered rather than quietly running everything.
func pytestArgs(paths []string, coverage bool, name string) (args []string, ok bool) {
	args = []string{"-q"}
	if coverage {
		args = append(args, "--cov=.")
	}
	if name != "" && !dashPattern(name) {
		args, ok = append(args, "-k", name), true
	}
	return append(args, paths...), ok
}

const pytestFilterWhy = `pytest -k takes its expression as a separate argv element, so a pattern beginning with "-" would be read as a flag`

func runPytest(root string, paths []string, coverage bool, name string) Result {
	r := Result{Ecosystem: "python", Coverage: -1}
	bin, err := exec.LookPath("pytest")
	if err != nil {
		return notRun(r, "pytest is not installed")
	}
	args, ok := pytestArgs(paths, coverage, name)
	r.Filtered = ok
	raw, err, timedOut := execute(root, bin, args)
	if timedOut {
		return notRun(r, "pytest gave no answer in "+runTimeout.String())
	}
	// exit 4 is pytest's usage error — with --cov that means pytest-cov is
	// missing; rerun without so the tests still count, honestly noted
	if coverage && err != nil && exitCode(err) == 4 {
		noCov, _ := pytestArgs(paths, false, name)
		raw, err, timedOut = execute(root, bin, noCov)
		if timedOut {
			return notRun(r, "pytest gave no answer in "+runTimeout.String())
		}
		defer func() { r.Detail += " — coverage not measured (pytest-cov not installed)" }()
		coverage = false
	}
	// exit 5 is pytest's "no tests collected": with a filter that means the
	// pattern matched nothing, which is a pass reported as such — never a
	// bare green implying the suite ran.
	if ok && err != nil && exitCode(err) == 5 {
		r.Verdict = Pass
		r.Detail = fmt.Sprintf("pass — 0 test(s) matched %q", name)
		return r
	}
	if m := pytestRe.FindStringSubmatch(raw); m != nil {
		r.Failed, _ = strconv.Atoi(m[1])
		r.Passed, _ = strconv.Atoi(m[2])
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = filterNote(fmt.Sprintf("FAILED (%d passed, %d failed) — %s", r.Passed, r.Failed, firstFailingLine(raw, errStr(err))), name, ok, pytestFilterWhy)
		return r
	}
	r.Verdict = Pass
	r.Detail = filterNote(fmt.Sprintf("pass (%d test(s))", r.Passed), name, ok, pytestFilterWhy)
	if ok && r.Passed == 0 {
		r.Detail = fmt.Sprintf("pass — 0 test(s) matched %q", name)
	}
	if coverage {
		if m := pytestCov.FindStringSubmatch(raw); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			r.Coverage = v
		}
	}
	return r
}

// gradleArgs builds the wrapper's argv. --tests takes its pattern as the
// next argv element, so a dash-leading pattern cannot be expressed.
func gradleArgs(name string) (args []string, ok bool) {
	args = []string{"test", "-q"}
	if name == "" || dashPattern(name) {
		return args, false
	}
	return append(args, "--tests", name), true
}

// mavenArgs builds mvn's argv. -Dtest= is a joined system property, so any
// pattern — dashes included — reaches surefire intact.
func mavenArgs(name string) (args []string, ok bool) {
	args = []string{"-q", "test"}
	if name == "" {
		return args, false
	}
	return append(args, "-Dtest="+name), true
}

const gradleFilterWhy = `gradle --tests takes its pattern as a separate argv element, so a pattern beginning with "-" would be read as a flag`

func runJava(root string, name string) Result {
	r := Result{Ecosystem: "java", Coverage: -1}
	if exists(root, "gradlew") {
		args, ok := gradleArgs(name)
		r.Filtered = ok
		raw, err, timedOut := execute(root, filepath.Join(root, "gradlew"), args)
		return javaVerdict(r, raw, err, timedOut, "gradlew test", name, ok, gradleFilterWhy)
	}
	if exists(root, "pom.xml") {
		bin, err := exec.LookPath("mvn")
		if err != nil {
			return notRun(r, "mvn is not installed")
		}
		args, ok := mavenArgs(name)
		r.Filtered = ok
		raw, rerr, timedOut := execute(root, bin, args)
		return javaVerdict(r, raw, rerr, timedOut, "mvn test", name, ok, "")
	}
	return notRun(r, "no gradle wrapper and no pom.xml runner available")
}

func javaVerdict(r Result, raw string, err error, timedOut bool, tool, name string, ok bool, why string) Result {
	if timedOut {
		return notRun(r, tool+" gave no answer in "+runTimeout.String())
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = filterNote("FAILED — "+firstFailingLine(raw, errStr(err)), name, ok, why)
		return r
	}
	r.Verdict = Pass
	r.Detail = filterNote("pass ("+tool+")", name, ok, why)
	return r
}

func execute(root, bin string, args []string) (string, error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // nosemgrep -- resolved from the fixed runner table, never user input
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err, ctx.Err() == context.DeadlineExceeded
}

// noSuite is "this ecosystem has nothing to run here" — a repository with
// a package.json and no test script has not failed a check, it has no JS
// suite. The distinction matters at the commit gate, which blocks on a
// check that could not run: without it, every commit in a repository like
// this one is refused because a runner it never asked for reported an
// absence.
func noSuite(r Result, why string) Result {
	r = notRun(r, why)
	r.NoSuite = true
	return r
}

func notRun(r Result, why string) Result {
	r.Verdict = NotRun
	// NOT run and NOT filtered are different answers: a runner that never
	// executed filtered nothing, so the flag goes back to false.
	r.Filtered = false
	r.Detail = "NOT run — " + why
	return r
}

// firstFailingLine prefers a line that names a failure over the first line
// of noise; a run that printed nothing still answers with the exit status.
func firstFailingLine(raw, errText string) string {
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		low := strings.ToLower(t)
		if t != "" && (strings.Contains(low, "fail") || strings.Contains(low, "error")) {
			return textutil.Trim(t)
		}
	}
	if l := textutil.FirstLine(raw); l != "no output" {
		return l
	}
	return "no output, exit status: " + errText
}

func exitCode(err error) int {
	if e, ok := err.(*exec.ExitError); ok {
		return e.ExitCode()
	}
	return -1
}

func cap3(names []string) []string {
	if len(names) > 3 {
		return append(names[:3], "…")
	}
	return names
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
