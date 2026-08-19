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
}

// Run executes every detected runner. paths narrow the Go package list and
// the pytest targets; other runners keep their native whole-project
// granularity (their Detail says so when paths were given).
func Run(root string, paths []string, coverage bool) []Result {
	var out []Result
	if exists(root, "go.mod") {
		out = append(out, runGo(root, paths, coverage))
	}
	if exists(root, "Cargo.toml") {
		out = append(out, runCargo(root))
	}
	if exists(root, "package.json") {
		out = append(out, runJS(root))
	}
	if pytestDetected(root) {
		out = append(out, runPytest(root, paths, coverage))
	}
	if exists(root, "build.gradle") || exists(root, "build.gradle.kts") || exists(root, "pom.xml") {
		out = append(out, runJava(root))
	}
	return out
}

// Report prints results and returns the command's exit code: 1 when any
// suite failed, 0 when at least one passed and none failed, 2 when nothing
// could run at all.
func Report(results []Result, out func(string)) int {
	if len(results) == 0 {
		out("NOT run — no recognized test setup in this repository (go.mod, Cargo.toml, package.json test script, pytest, gradle/maven)")
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
		results := Run(root, nil, false)
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

func runGo(root string, paths []string, coverage bool) Result {
	r := Result{Ecosystem: "go", Coverage: -1}
	bin, err := exec.LookPath("go")
	if err != nil {
		return notRun(r, "the go toolchain is not installed")
	}
	args := []string{"test"}
	if coverage {
		args = append(args, "-cover")
	}
	args = append(args, goTargets(root, paths)...)
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
			detail = "FAILED — " + firstLine(raw+errStr(err))
		}
		r.Verdict, r.Detail, r.Failed = Fail, detail, len(names)
		return r
	}
	pkgs := len(goOkPkgRe.FindAllString(raw, -1))
	r.Verdict = Pass
	r.Detail = fmt.Sprintf("pass (%d package(s))", pkgs)
	if strings.Contains(raw, "[no test files]") && pkgs == 0 {
		r.Detail = "pass — but no test files exist yet"
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

func runCargo(root string) Result {
	r := Result{Ecosystem: "rust", Coverage: -1}
	bin, err := exec.LookPath("cargo")
	if err != nil {
		return notRun(r, "cargo is not installed")
	}
	raw, err, timedOut := execute(root, bin, []string{"test", "--quiet"})
	if timedOut {
		return notRun(r, "cargo test gave no answer in "+runTimeout.String())
	}
	if m := cargoSumRe.FindStringSubmatch(raw); m != nil {
		r.Passed, _ = strconv.Atoi(m[2])
		r.Failed, _ = strconv.Atoi(m[3])
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = fmt.Sprintf("FAILED (%d passed, %d failed) — %s", r.Passed, r.Failed, firstLine(raw+errStr(err)))
		return r
	}
	r.Verdict = Pass
	r.Detail = fmt.Sprintf("pass (%d test(s))", r.Passed)
	return r
}

func runJS(root string) Result {
	r := Result{Ecosystem: "js", Coverage: -1}
	script := testScript(root)
	if script == "" {
		return notRun(r, "package.json has no test script")
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
	raw, err, timedOut := execute(root, bin, []string{"test"})
	if timedOut {
		return notRun(r, mgr+" test gave no answer in "+runTimeout.String())
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = "FAILED — " + firstFailingLine(raw, errStr(err))
		return r
	}
	r.Verdict = Pass
	r.Detail = "pass (" + mgr + " test)"
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

func runPytest(root string, paths []string, coverage bool) Result {
	r := Result{Ecosystem: "python", Coverage: -1}
	bin, err := exec.LookPath("pytest")
	if err != nil {
		return notRun(r, "pytest is not installed")
	}
	args := []string{"-q"}
	if coverage {
		args = append(args, "--cov=.")
	}
	args = append(args, paths...)
	raw, err, timedOut := execute(root, bin, args)
	if timedOut {
		return notRun(r, "pytest gave no answer in "+runTimeout.String())
	}
	// exit 4 is pytest's usage error — with --cov that means pytest-cov is
	// missing; rerun without so the tests still count, honestly noted
	if coverage && err != nil && exitCode(err) == 4 {
		raw, err, timedOut = execute(root, bin, append([]string{"-q"}, paths...))
		if timedOut {
			return notRun(r, "pytest gave no answer in "+runTimeout.String())
		}
		defer func() { r.Detail += " — coverage not measured (pytest-cov not installed)" }()
		coverage = false
	}
	if m := pytestRe.FindStringSubmatch(raw); m != nil {
		r.Failed, _ = strconv.Atoi(m[1])
		r.Passed, _ = strconv.Atoi(m[2])
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = fmt.Sprintf("FAILED (%d passed, %d failed) — %s", r.Passed, r.Failed, firstFailingLine(raw, errStr(err)))
		return r
	}
	r.Verdict = Pass
	r.Detail = fmt.Sprintf("pass (%d test(s))", r.Passed)
	if coverage {
		if m := pytestCov.FindStringSubmatch(raw); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			r.Coverage = v
		}
	}
	return r
}

func runJava(root string) Result {
	r := Result{Ecosystem: "java", Coverage: -1}
	if exists(root, "gradlew") {
		raw, err, timedOut := execute(root, filepath.Join(root, "gradlew"), []string{"test", "-q"})
		return javaVerdict(r, raw, err, timedOut, "gradlew test")
	}
	if exists(root, "pom.xml") {
		bin, err := exec.LookPath("mvn")
		if err != nil {
			return notRun(r, "mvn is not installed")
		}
		raw, rerr, timedOut := execute(root, bin, []string{"-q", "test"})
		return javaVerdict(r, raw, rerr, timedOut, "mvn test")
	}
	return notRun(r, "no gradle wrapper and no pom.xml runner available")
}

func javaVerdict(r Result, raw string, err error, timedOut bool, tool string) Result {
	if timedOut {
		return notRun(r, tool+" gave no answer in "+runTimeout.String())
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = "FAILED — " + firstFailingLine(raw, errStr(err))
		return r
	}
	r.Verdict = Pass
	r.Detail = "pass (" + tool + ")"
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

func notRun(r Result, why string) Result {
	r.Verdict = NotRun
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
			return trim160(t)
		}
	}
	if l := firstLine(raw); l != "no output" {
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

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			return trim160(t)
		}
	}
	return "no output"
}

func trim160(s string) string {
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
