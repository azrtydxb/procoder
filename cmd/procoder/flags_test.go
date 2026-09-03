package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"procoder/internal/security"
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
	if !strings.Contains(errb.String(), "--paths-from") {
		t.Errorf("the refusal must say what check does take: %q", errb.String())
	}
}

// The other half of the same message. `check` used to be the example of a
// command taking no flags at all, and stopped being one when --paths-from
// landed — so the "no flags" wording needs a command that still is, or the
// branch goes untested the moment any command gains its first flag.
//
// proved by: change the empty-allowed branch in checkFlags to print the
// same sentence as the non-empty one — this fails.
func TestACommandWithNoFlagsSaysSo(t *testing.T) {
	var errb bytes.Buffer
	if _, ok := checkFlags([]string{"doctor", "--nope"}, &errb); ok {
		t.Fatal("doctor accepted --nope, which it does not implement")
	}
	if !strings.Contains(errb.String(), "no flags") {
		t.Errorf("a command with no flags must say so: %q", errb.String())
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

	if code := run([]string{"config", "--get"}, processSession()); code != 2 {
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

// `procoder security` is how a person checks their work before committing,
// so it has to ask what the gate asks. It did not: the gate runs the
// secret scanner AND the SAST leg over the changed files, and the command
// ran only the secret scanner. A hardcoded AWS key blocked at the gate and
// was reported here as "0 finding(s) (0 blocking)" — the most dangerous
// place in procoder to answer a question it did not ask.
//
// gitleaks is what makes this reachable rather than theoretical: it does
// not fire on a bare `const K = "AKIA…"` in Go, and semgrep does, so the
// two legs genuinely differ in what they see.
//
// proved by: deleting the SastChanged call from the non-deep security arm
// — this test then finds the command silent about a key the gate blocks.
func TestSecurityAsksWhatTheGateAsks(t *testing.T) {
	gateSrc, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(gateSrc)
	arm := src[strings.Index(src, `case "security":`):]
	arm = arm[:strings.Index(arm, `case "lint":`)]

	for _, leg := range []string{"SecretsChangedFiles", "SastChanged"} {
		if !strings.Contains(arm, leg) {
			t.Errorf("the security command does not run %s, which the gate runs over the same files", leg)
		}
	}
	// --deep replaces the changed-file SAST leg with the whole-tree one;
	// running both would report every finding twice.
	if strings.Count(arm, "SastChanged") != 1 {
		t.Errorf("SastChanged belongs in the non-deep branch only: %s", arm)
	}
}

// `procoder security scripts/` scanned the change set and answered about
// that, so a person who named a directory was told "0 finding(s)" about
// files nobody had opened. The empty-change-set message added earlier in
// the same campaign ends "(pass paths to check them anyway)" — advice this
// command did not take, which made the silence worse than accidental.
//
// A directory has to be expanded too: SecretsChangedFiles drops anything
// that is not a regular file, so a bare directory scans nothing and
// reports clean — the same trap lintCmd already carries a comment about.
//
// proved by: deleting the expandDirs call from the security arm — the
// directory case then finds nothing.
func TestSecurityHonoursThePathsItIsGiven(t *testing.T) {
	root := repoRootForTest(t)
	if _, err := exec.LookPath("gitleaks"); err != nil {
		t.Skip("gitleaks is not installed")
	}
	dir := filepath.Join(root, "internal", "e2eprobe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	// Derived rather than written: this file must not be a credential
	// literal itself, and a hand-invented string does not reliably look
	// like a key to the scanner — gitleaks weighs the body's character
	// profile, and the first hand-written attempt here was not flagged at
	// all, which would have made this test pass for the wrong reason.
	sum := sha256.Sum256([]byte("procoder-e2e-flag-test"))
	body := make([]byte, 0, 16)
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	for _, b := range sum {
		if len(body) == 16 {
			break
		}
		body = append(body, alphabet[int(b)%len(alphabet)])
	}
	key := "AKIA" + string(body)
	if err := os.WriteFile(filepath.Join(dir, "p.go"),
		[]byte("package e2eprobe\n\nconst K = \""+key+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, arg := range []string{"internal/e2eprobe/p.go", "internal/e2eprobe"} {
		got := security.SecretsChangedFiles(root, expandDirs(root, []string{arg}))
		if len(got) == 0 {
			t.Errorf("a planted credential under %q was not found", arg)
			continue
		}
		for _, f := range got {
			if strings.Contains(f.Message, key) {
				t.Errorf("the finding echoed the credential's value: %q", f.Message)
			}
		}
	}
}

// proved by: returning paths unchanged from expandDirs — the directory
// then contributes nothing and this test sees no files.
func TestADirectoryArgumentBecomesItsFiles(t *testing.T) {
	root := repoRootForTest(t)
	got := expandDirs(root, []string{"internal/audit"})
	if len(got) < 2 {
		t.Fatalf("internal/audit holds several files; expandDirs gave %d: %q", len(got), got)
	}
	for _, g := range got {
		if strings.HasSuffix(g, "internal/audit") {
			t.Errorf("the directory itself survived expansion: %q", got)
		}
	}
}

// proved by: dropping the HasPrefix check — the flag is then treated as a
// path and handed to the scanner as a filename, which is the bug the flag
// guard was written for, reappearing one layer down.
func TestPositionalDropsFlagsAndKeepsPaths(t *testing.T) {
	got := positional([]string{"--deep", "internal", "cmd/procoder"})
	if len(got) != 2 || got[0] != "internal" || got[1] != "cmd/procoder" {
		t.Errorf("positional() = %q, want the two paths without the flag", got)
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}
