package testrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The output below is recorded from phpunit 13.3, not invented.

// A suite where tests both failed and errored must count both: a reader
// told "1 failed" when two more tests errored is being undercounted, and
// the number is the whole point of the line.
// proved by: dropped the Errors term from phpunitCounts — a run with one
// failure and one error then reports 2 passed, 1 failed over a suite of
// three, and the errored test is counted as having passed.
func TestAFailedRunCountsErrorsAsWellAsFailures(t *testing.T) {
	raw := `.FE                                                                 3 / 3 (100%)

There was 1 error:

1) MathTest::testErrors
RuntimeException: boom

FAILURES!
Tests: 3, Assertions: 2, Errors: 1, Failures: 1.`
	passed, failed := phpunitCounts(raw)
	if passed != 1 || failed != 2 {
		t.Errorf("want 1 passed and 2 not passing, got %d and %d", passed, failed)
	}
}

// The all-passed line is a different shape from the summary, and it is the
// only place the count appears when nothing failed.
// proved by: removed the phpunitOK branch from phpunitCounts — a fully
// passing run then reports 0 passed, which reads as a suite that did not
// run.
func TestAPassingRunReportsItsCount(t *testing.T) {
	passed, failed := phpunitCounts("OK (2 tests, 2 assertions)")
	if passed != 2 || failed != 0 {
		t.Errorf("want 2 passed and 0 failed, got %d and %d", passed, failed)
	}
}

// --filter takes its pattern as the next argv element, so a pattern
// beginning with a dash would be read as a flag. The run must then be
// reported as NOT filtered rather than wearing a filtered label over a
// whole-suite run.
// proved by: passed the dash pattern through — phpunit reads "-x" as a
// flag, the whole suite runs, and the report calls it filtered.
func TestADashPatternIsNotSilentlyPassedToPhpunit(t *testing.T) {
	if _, filtered := phpunitArgs("-x"); filtered {
		t.Error("a pattern beginning with a dash must not be reported as filtered")
	}
	args, filtered := phpunitArgs("testPasses")
	if !filtered || len(args) != 2 || args[0] != "--filter" || args[1] != "testPasses" {
		t.Errorf("an ordinary pattern must reach --filter, got %v filtered=%v", args, filtered)
	}
	if _, filtered := phpunitArgs(""); filtered {
		t.Error("no pattern is not a filtered run")
	}
}

// Detection is the config file, in either spelling: a PHP project commits
// phpunit.xml.dist as the shared default and lets a developer override it
// with an untracked phpunit.xml.
// proved by: matched phpunit.xml only — a project shipping the .dist file,
// which is the common case, is told it has no recognized test setup.
func TestPhpunitIsDetectedByEitherConfigSpelling(t *testing.T) {
	for _, name := range []string{"phpunit.xml", "phpunit.xml.dist"} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), []byte("<phpunit/>"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !phpunitDetected(root) {
			t.Errorf("%s must select the phpunit runner", name)
		}
	}
	if phpunitDetected(t.TempDir()) {
		t.Error("a directory with no phpunit config is not a phpunit project")
	}
}

// Coverage needs xdebug or pcov in the PHP runtime, which procoder cannot
// assume and does not install. Saying so is the honest answer; a number it
// did not measure would not be.
// proved by: reported a coverage figure of 0 instead — `test --coverage`
// then shows 0% for a suite whose coverage was never measured.
func TestCoverageIsReportedNotMeasured(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "phpunit.xml"), []byte("<phpunit/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runPHPUnit(root, true, "")
	if r.Coverage >= 0 {
		t.Errorf("coverage must be reported as not measured, got %v", r.Coverage)
	}
	// Whatever else happened (phpunit may not be installed here), the
	// result must never claim a coverage number.
	if strings.Contains(r.Detail, "%") {
		t.Errorf("no percentage may appear: %q", r.Detail)
	}
}
