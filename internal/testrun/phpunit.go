package testrun

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// phpunitDetected reports whether this repository's suite is phpunit. The
// config file is the signal, in either spelling: PHP projects commit
// phpunit.xml.dist as the shared default and let a developer override it
// with an untracked phpunit.xml.
func phpunitDetected(root string) bool {
	return exists(root, "phpunit.xml") || exists(root, "phpunit.xml.dist")
}

// phpunitBin finds the runner the project installed. composer puts it in
// vendor/bin, which is where nearly every PHP project has it; a global
// install is the fallback, not the expectation.
func phpunitBin(root string) string {
	local := filepath.Join(root, "vendor", "bin", "phpunit")
	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local
	}
	if p, err := exec.LookPath("phpunit"); err == nil {
		return p
	}
	return ""
}

var (
	// `OK (2 tests, 2 assertions)` — the all-passed line.
	phpunitOK = regexp.MustCompile(`OK \((\d+) test`)
	// `Tests: 5, Assertions: 5, Failures: 1, Errors: 2.` — the summary
	// printed whenever anything did not pass. Errors and failures are
	// counted together: both mean the suite did not pass, and a reader
	// being told "1 failure" when two more tests errored is being
	// undercounted.
	phpunitTests    = regexp.MustCompile(`Tests: (\d+)`)
	phpunitFailures = regexp.MustCompile(`Failures: (\d+)`)
	phpunitErrors   = regexp.MustCompile(`Errors: (\d+)`)
)

// phpunitFilterWhy explains the one case where --name cannot be honoured.
const phpunitFilterWhy = `phpunit --filter takes its pattern as the next argv element, so a pattern beginning with "-" would be read as a flag`

// phpunitArgs builds phpunit's argv. --filter takes its pattern as a
// separate element, so a pattern starting with a dash would be read as a
// flag — the same trap pytest's -k has, and reported the same way rather
// than silently running the whole suite under a filtered label.
func phpunitArgs(name string) (args []string, filtered bool) {
	if name == "" || strings.HasPrefix(name, "-") {
		return nil, false
	}
	return []string{"--filter", name}, true
}

// runPHPUnit runs the project's phpunit and reports what it said.
//
// Coverage is reported NOT measured rather than requested: phpunit's
// coverage needs xdebug or pcov present and enabled in the PHP runtime, and
// asking for it without them makes phpunit fail the whole run. A number
// procoder did not measure is worse than saying it measured none.
func runPHPUnit(root string, coverage bool, name string) Result {
	r := Result{Ecosystem: "php", Coverage: -1}
	bin := phpunitBin(root)
	if bin == "" {
		return notRun(r, "phpunit is not installed — composer require --dev phpunit/phpunit")
	}
	args, filtered := phpunitArgs(name)
	r.Filtered = filtered
	raw, err, timedOut := execute(root, bin, args)
	if timedOut {
		return notRun(r, "phpunit gave no answer in "+runTimeout.String())
	}

	r.Passed, r.Failed = phpunitCounts(raw)

	detail := ""
	if coverage {
		detail = " — coverage not measured (phpunit needs xdebug or pcov in the PHP runtime)"
	}
	if err != nil {
		r.Verdict = Fail
		r.Detail = filterNote(fmt.Sprintf("FAILED (%d passed, %d failed) — %s",
			r.Passed, r.Failed, firstFailingLine(raw, errStr(err))), name, filtered, phpunitFilterWhy) + detail
		return r
	}
	r.Verdict = Pass
	r.Detail = filterNote(fmt.Sprintf("pass (%d test(s))", r.Passed), name, filtered, phpunitFilterWhy) + detail
	if filtered && r.Passed == 0 {
		r.Detail = fmt.Sprintf("pass — 0 test(s) matched %q", name) + detail
	}
	return r
}

// phpunitCounts reads the pass and fail totals out of a phpunit run.
//
// Errors count as failures: both mean the test did not pass, and a reader
// told "1 failed" when two more tests errored is being undercounted. The
// two shapes are separate because phpunit prints the count in one place
// when everything passed and in another when anything did not.
func phpunitCounts(raw string) (passed, failed int) {
	if m := phpunitOK.FindStringSubmatch(raw); m != nil {
		p, _ := strconv.Atoi(m[1])
		return p, 0
	}
	m := phpunitTests.FindStringSubmatch(raw)
	if m == nil {
		return 0, 0
	}
	total, _ := strconv.Atoi(m[1])
	failed = countOf(phpunitFailures, raw) + countOf(phpunitErrors, raw)
	passed = total - failed
	if passed < 0 {
		passed = 0
	}
	return passed, failed
}

func countOf(re *regexp.Regexp, raw string) int {
	if m := re.FindStringSubmatch(raw); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}
