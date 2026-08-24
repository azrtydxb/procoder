package main

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The defect this file exists for: `procoder check --staged` exited 0. The
// arm passes args[1:] to the gate as paths, no formatter covers a file
// called "--staged", so it was counted out of scope, nothing else was
// looked at, and the gate reported clean. A gate that passes because it
// checked a typo is a silent green in the command that enforces them.
//
// proved by: replacing `!contains(allowed, a)` with `false` in
// checkFlags — this test then reports that --staged was accepted.
func TestAFlagTheCommandDoesNotImplementIsRefused(t *testing.T) {
	var errb bytes.Buffer
	if _, ok := checkFlags([]string{"check", "--staged"}, &errb); ok {
		t.Fatal("check accepted --staged, which it does not implement")
	}
	if !strings.Contains(errb.String(), "--staged") {
		t.Errorf("the refusal must name the flag: %q", errb.String())
	}
	if !strings.Contains(errb.String(), "no flags") {
		t.Errorf("the refusal must say what check does take: %q", errb.String())
	}
}

// proved by: returning an empty slice from knownFlags lookups — every one
// of these then fails.
func TestEveryFlagACommandImplementsIsAccepted(t *testing.T) {
	for cmd, flags := range knownFlags {
		for _, f := range flags {
			var errb bytes.Buffer
			if _, ok := checkFlags([]string{cmd, f}, &errb); !ok {
				t.Errorf("%s refused its own flag %s: %s", cmd, f, errb.String())
			}
		}
	}
}

// A word beginning with a dash inside free text is a word. `docs --ack`
// takes a commit message and `sprint carry` takes a reason; refusing those
// would trade one usage bug for another.
//
// proved by: removing the `return true` on the first positional — the
// commit-message case then fails.
func TestScanningStopsAtTheFirstPositional(t *testing.T) {
	var errb bytes.Buffer
	if _, ok := checkFlags([]string{"docs", "--ack", "fix: -n was never read"}, &errb); !ok {
		t.Errorf("a dash inside the acknowledgment message was read as a flag: %s", errb.String())
	}
	errb.Reset()
	if _, ok := checkFlags([]string{"sprint", "carry", "story-1", "--blocked on review"}, &errb); !ok {
		t.Errorf("a dash inside a carry reason was read as a flag: %s", errb.String())
	}
	errb.Reset()
	rest, ok := checkFlags([]string{"check", "--", "--weirdly-named-file"}, &errb)
	if !ok {
		t.Errorf("-- did not end flag scanning: %s", errb.String())
	}
	// The separator is consumed, not forwarded: `procoder check -- x.go`
	// used to report a file named "--" as out of scope.
	want := []string{"check", "--weirdly-named-file"}
	if len(rest) != len(want) || rest[0] != want[0] || rest[1] != want[1] {
		t.Errorf("-- was forwarded to the command: %q", rest)
	}
}

// The table is hand-maintained, so it drifts the moment somebody adds a
// flag to an arm and not to it — and the drift is silent, because the new
// flag simply gets refused. This holds the two together: every flag
// literal main.go compares an argument against must be in the table.
//
// proved by: deleting "--deep" from the security row — this test then
// names it as implemented but not listed.
func TestNoImplementedFlagIsMissingFromTheTable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, flags := range knownFlags {
		for _, f := range flags {
			listed[f] = true
		}
	}
	// The arms read flags by comparing an argument against a literal:
	// `args[1] == "--deep"`, `case args[i] == "--coverage":`,
	// `flagVal("--epic", ...)`. Any of those shapes is a flag the CLI
	// implements. `git config --get` is not one of them — it is an
	// argument to git — so the pattern requires the literal to be
	// compared against or looked up, not merely present.
	pat := regexp.MustCompile(`(?:==\s*|flagVal\()"(--[a-z][a-z-]*)"`)
	for _, m := range pat.FindAllStringSubmatch(string(src), -1) {
		if !listed[m[1]] {
			t.Errorf("main.go implements %s but knownFlags does not list it — it would be refused", m[1])
		}
	}
}

// checkFlags is only worth having if run() calls it, and the first version
// of this file tested the helper alone — so deleting the call from run()
// left every test passing. This one goes through the dispatch.
//
// `config` takes no flags and reports from local files, so it is cheap
// enough to invoke and cannot pass for an unrelated reason.
//
// proved by: deleting the checkFlags call from run() in main.go — this
// test then sees exit 0 where the usage error should be.
func TestTheDispatchItselfRefusesTheFlag(t *testing.T) {
	stderr := os.Stderr
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = devnull
	defer func() { os.Stderr = stderr; devnull.Close() }()

	if code := run([]string{"config", "--get"}); code != 2 {
		t.Errorf("run(config --get) = %d, want 2 (a flag config does not implement is a usage error)", code)
	}
}

// `procoder adr` with no subcommand printed all 159 lines of the usage
// text, leaving somebody who forgot one word to find their command among
// seventy-eight. The usage error now answers the question that was asked.
//
// proved by: making usageFor return `usage` unconditionally — this test
// then sees the whole text where one block belongs.
func TestAUsageErrorAnswersAboutTheCommandThatWasTyped(t *testing.T) {
	got := usageFor("adr")

	if !strings.Contains(got, "architecture decision records") {
		t.Errorf("adr's own description is missing:\n%s", got)
	}
	if strings.Contains(got, "the commit gate") {
		t.Errorf("adr's usage carries the whole book:\n%s", got)
	}
	if n := strings.Count(got, "\n"); n > 12 {
		t.Errorf("one command's usage is %d lines, which is the bug this replaced", n)
	}
	// The continuation lines are what make the block readable, so they
	// have to come with it.
	if !strings.Contains(got, "new <title> | list | check") {
		t.Errorf("the block stopped at its first line:\n%s", got)
	}
}

// A command name procoder does not have is a different question — there
// the whole list IS the answer.
//
// proved by: returning "" instead of usage for an unmatched name — this
// test then finds no command list to fall back to.
func TestAnUnknownCommandStillGetsTheWholeList(t *testing.T) {
	got := usageFor("thereisnosuchcommand")
	if !strings.Contains(got, "the commit gate") {
		t.Errorf("a mistyped command name needs the list of real ones:\n%s", got)
	}
}
