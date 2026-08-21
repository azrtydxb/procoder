package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The output below is recorded from the real tools (phpstan 2.2.8, php
// 8.5.6), not invented: the parsers are what this test exists to hold, and
// a parser tested against output nobody has seen proves the parser matches
// the author's guess.

// phpstan's raw format writes `path:line:message` with NO space after the
// line number, and the shared finding parser requires that space. Reusing
// it would drop every phpstan finding and report a clean lint over a file
// full of errors — a false green, which is the one verdict procoder must
// never produce.
// proved by: required the space in phpstanLine (`:(\d+):\s+(.+)$`, which
// is what the shared parser matches) — every finding below disappears and
// a file full of type errors reports clean.
func TestPhpstanFindingsSurviveTheMissingSpace(t *testing.T) {
	raw := `Note: Using configuration file /work/phpstan.neon.
/work/bug.php:3:Function add() should return int but returns string. [identifier=return.type]
/work/bug.php:5:Function undefinedFunc not found. [identifier=function.notFound]
 [ERROR] Found 2 errors`
	got := parsePhpstan(raw, true)
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %d: %+v", len(got), got)
	}
	if got[0].File != "/work/bug.php" || got[0].Line != 3 {
		t.Errorf("first finding must carry file and line, got %s:%d", got[0].File, got[0].Line)
	}
	// The identifier names the rule, not the problem. The reader gets the
	// sentence; a message ending in "[identifier=return.type]" reads as
	// though the tool leaked its internals.
	if strings.Contains(got[0].Message, "identifier=") {
		t.Errorf("the rule identifier must not reach the reader: %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "should return int") {
		t.Errorf("the message must survive: %q", got[0].Message)
	}
	if !got[0].Blocking {
		t.Error("block must be honoured")
	}
}

// A file that parses produces no finding at all: php -l has nothing to say
// about style, and a repository that configured no linter did not ask for
// an opinion.
// proved by: made lintPHPSyntax report on exit 0 too — every clean PHP file
// then carries "No syntax errors detected" as a finding.
func TestPHPSyntaxErrorsAreReportedAndCleanFilesAreSilent(t *testing.T) {
	raw := `PHP Parse error:  syntax error, unexpected token "{", expecting variable in /work/broken.php on line 2

Parse error: syntax error, unexpected token "{", expecting variable in /work/broken.php on line 2`
	got := parsePHPSyntax(raw, "/work/broken.php", true)
	if len(got) != 1 {
		t.Fatalf("a parse error is one finding, not one per echoed line: got %d", len(got))
	}
	if got[0].Line != 2 || got[0].File != "/work/broken.php" {
		t.Errorf("want /work/broken.php:2, got %s:%d", got[0].File, got[0].Line)
	}
	if strings.Contains(got[0].Message, "Parse error") {
		t.Errorf("the label is not the message: %q", got[0].Message)
	}

	// php exited non-zero saying something unrecognised. Unknown is not
	// clean — it must surface as NOT checked.
	unknown := parsePHPSyntax("something nobody has seen", "/work/x.php", true)
	if len(unknown) != 1 || !strings.Contains(unknown[0].Message, "NOT checked") {
		t.Errorf("an unreadable failure must be NOT checked, got %+v", unknown)
	}
}

// The linter is whichever the project configured. Committed .dist variants
// count: that is how a PHP project ships a shared default.
// proved by: dropped the .dist spellings — a project carrying only
// phpstan.neon.dist then falls through to the syntax floor and its
// configured analysis never runs.
func TestTheConfiguredLinterIsFound(t *testing.T) {
	for _, name := range []string{"phpstan.neon", "phpstan.neon.dist", "phpcs.xml", "phpcs.xml.dist"} {
		root := t.TempDir()
		deep := filepath.Join(root, "src", "app")
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Found by walking up from the FILE, not by looking at the root
		// only: a monorepo puts a package's config beside that package.
		file := filepath.Join(deep, "Thing.php")
		set := phpstanConfigs
		if strings.HasPrefix(name, "phpcs") {
			set = phpcsConfigs
		}
		if got := hasAny(root, file, set); got == "" {
			t.Errorf("%s must select its linter, walking up from %s", name, file)
		}
	}

	// Neither config: nothing selected, and the caller falls to php -l.
	root := t.TempDir()
	if got := hasAny(root, filepath.Join(root, "a.php"), phpstanConfigs); got != "" {
		t.Errorf("an unconfigured project selects no linter, got %q", got)
	}
}

// phpcs reports `path:line:col: level - message`, which the shared finding
// parser already reads. The test exists because "already reads it" is a
// claim about a regex meeting output neither was written for, and the cost
// of being wrong is a configured linter reporting nothing.
// proved by: narrowed findingLine to require no column (`^(.+?):(\d+):\s`
// without the optional `:\d+`) — every phpcs finding then vanishes and a
// PSR-12 violation reports clean.
func TestPhpcsFindingsAreRead(t *testing.T) {
	raw := `/work/bug.php:1:1: warning - A file should declare new symbols or execute logic, but not both.
/work/bug.php:2:35: error - Opening brace should be on a new line`
	got := parse(raw, true)
	if len(got) != 2 {
		t.Fatalf("want 2 phpcs findings, got %d: %+v", len(got), got)
	}
	if got[1].File != "/work/bug.php" || got[1].Line != 2 {
		t.Errorf("want /work/bug.php:2, got %s:%d", got[1].File, got[1].Line)
	}
	if !strings.Contains(got[1].Message, "Opening brace") {
		t.Errorf("the message must survive: %q", got[1].Message)
	}
}

// With no php on PATH the floor cannot run, and a linter that cannot run
// has NOT checked the file. Reporting nothing would be indistinguishable
// from a clean file — the false green procoder exists to prevent.
// proved by: returned nil instead of notChecked when php is missing — a
// machine without php then reports every PHP file clean.
func TestWithoutPHPTheFloorSaysNotChecked(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.php")
	if err := os.WriteFile(file, []byte("<?php\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// PATH is an empty directory and nothing else: a php installed on this
	// machine would otherwise answer, and the test would stop testing
	// anything without ever going red.
	t.Setenv("PATH", t.TempDir())
	got := lintPHP(root, []string{file}, true)
	if len(got) == 0 {
		t.Fatal("a file nothing could check must never be silently clean")
	}
	// The count is deliberately not asserted: with no config procoder also
	// reports its default linter missing, and pinning the number here would
	// make this test fail every time the set of things it reports changes,
	// for a reason that has nothing to do with what it is guarding.
	var named bool
	for _, f := range got {
		if strings.Contains(f.Message, "NOT checked") && strings.Contains(f.Message, "php") {
			named = true
		}
	}
	if !named {
		t.Errorf("the refusal must say it did not check, and name php: %+v", got)
	}
}

// A PHP project that configured nothing must still be linted. Before this
// it fell to `php -l`, which reports syntax errors and nothing else — so a
// wrong return type or a call to a function that does not exist passed the
// gate on every unconfigured PHP repository, which is most of them on the
// day procoder arrives.
// proved by: pointed the no-config branch back at lintPHPSyntax — the
// baseline never runs, and a project with no config gets a syntax check
// wearing the name of a linter.
func TestAnUnconfiguredProjectStillGetsARealLinter(t *testing.T) {
	// phpstan absent is the case this can assert without the tool: the
	// refusal must name phpstan, so `procoder init` knows what to install,
	// AND still run the syntax floor rather than leaving the file unread.
	root := t.TempDir()
	file := filepath.Join(root, "a.php")
	if err := os.WriteFile(file, []byte("<?php\nfunction f( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir()) // neither phpstan nor php can answer
	got := lintPHP(root, []string{file}, true)
	if len(got) == 0 {
		t.Fatal("an unconfigured project must not be silently clean")
	}
	var named bool
	for _, f := range got {
		if strings.Contains(f.Message, "phpstan") && strings.Contains(f.Message, "NOT checked") {
			named = true
		}
	}
	if !named {
		t.Errorf("the missing default linter must be named so init can install it: %+v", got)
	}
}

// The baseline is level 5, and the level is the whole decision: it must
// catch what is a bug in any style without demanding a typed codebase.
// Level 6 requires a typehint on every parameter, which shouts at every
// existing PHP project on the day procoder is installed — and a default
// people turn off protects nobody.
// proved by: set the baseline to level 6 — this test names it, and the
// measurement behind it (four findings on fourteen lines of ordinary
// untyped PHP, where level 5 reports none) is in the comment above it.
func TestTheBaselineLevelIsTheMeasuredOne(t *testing.T) {
	if !strings.Contains(phpstanBaseline, "level: 5") {
		t.Errorf("the curated baseline must be level 5:\n%s", phpstanBaseline)
	}
	// It is a config procoder writes to a temp file, never into the repo:
	// a baseline that appeared as phpstan.neon in someone's tree would be
	// procoder imposing a config it promised not to impose.
	if strings.Contains(phpstanBaseline, "paths:") {
		t.Error("the baseline must not pin paths — the files come from the command line")
	}
}
