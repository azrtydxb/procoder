package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"procoder/internal/answers"
	"procoder/internal/ask"
	"procoder/internal/releases"
)

var (
	testSum     = strings.Repeat("ab", 32)
	testPending = releases.NewPending("1.0.0", "1.1.0", "procoder-test",
		"https://github.com/azrtydxb/procoder/releases/download/v1.1.0/procoder-test", testSum)
)

// noTerminal is a session with nobody at the other end — an agent's shell —
// in root.
func noTerminal(root string) (session, *bytes.Buffer) {
	var out bytes.Buffer
	return session{stdout: &out, stderr: &out, stdin: strings.NewReader(""), cwd: root}, &out
}

// governedRepo is a repository procoder governs: it has a .procoder/.
func governedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func decisionsText(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ask.Dir), ask.DecisionsFile))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(raw)
}

// The handshake: no terminal installs nothing, and prints the versions, the
// URL, the digest and the exact command that installs that digest.
// proved by: returned Install: true from the no-terminal path — the
// upgrade then proceeds on nobody's say-so.
func TestNoTerminalPrintsTheHandshakeAndInstallsNothing(t *testing.T) {
	s, out := noTerminal(t.TempDir())
	got := s.selfUpgradeConsent(false, "")(testPending)
	if got.Install {
		t.Fatalf("no terminal must not consent: %+v", got)
	}
	for _, want := range []string{
		"pending upgrade: procoder 1.0.0 → 1.1.0",
		"asset:  " + testPending.URL,
		"sha256: " + testSum,
		"procoder self-upgrade --confirm " + testSum,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the handshake must print %q:\n%s", want, out)
		}
	}
	// Not governed: no decision is written anywhere.
	if strings.Contains(out.String(), "decisions.md") {
		t.Errorf("an ungoverned directory gets no decision:\n%s", out)
	}
}

// --force travels into the printed command, or the command the person runs
// is refused by the very check they already chose to pass.
func TestTheHandshakeKeepsForce(t *testing.T) {
	s, out := noTerminal(t.TempDir())
	s.selfUpgradeConsent(true, "")(testPending)
	if !strings.Contains(out.String(), "procoder self-upgrade --force --confirm "+testSum) {
		t.Errorf("a --force run must print a --force confirmation:\n%s", out)
	}
}

// --confirm is a pin, not a switch.
// proved by: returned Consent{Install: true} without the pin — a confirmed
// hash then confirms whatever the release serves.
func TestConfirmPinsTheDigest(t *testing.T) {
	s, _ := noTerminal(t.TempDir())
	got := s.selfUpgradeConsent(false, testSum)(testPending)
	if !got.Install || got.Pinned != testSum {
		t.Errorf("--confirm must consent pinned to its digest: %+v", got)
	}
}

// In a governed repository the pending upgrade becomes a decision, written
// once however many times the agent re-runs the command.
// proved by: skipped the RaiseDecision call — the run still says "recorded
// as a decision" and the file holds no heading at all. (The duplicate guard
// itself is proved in internal/ask's TestRaiseDecisionWritesOnce.)
func TestTheDecisionIsWrittenOnce(t *testing.T) {
	root := governedRepo(t)
	for i := 0; i < 3; i++ {
		s, out := noTerminal(root)
		if got := s.selfUpgradeConsent(false, "")(testPending); got.Install {
			t.Fatalf("an unanswered decision must not install: %+v", got)
		}
		want := "recorded as a decision"
		if i > 0 {
			want = "already waiting in"
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("run %d must say %q:\n%s", i+1, want, out)
		}
	}
	text := decisionsText(t, root)
	heading := "## Upgrade procoder 1.0.0 → 1.1.0 (sha256 " + testSum + ")?"
	if strings.Count(text, heading) != 1 {
		t.Errorf("the decision heading must appear exactly once:\n%s", text)
	}
	for _, option := range []string{"- install:", "- skip:"} {
		if !strings.Contains(text, option) {
			t.Errorf("the decision must offer %q:\n%s", option, text)
		}
	}
}

// answer records an answer to the upgrade decision the way `procoder ask`
// does: under the decision question's own key.
func answer(t *testing.T, root, said string) {
	t.Helper()
	pending, _, err := ask.PendingDecisions(root)
	if err != nil || len(pending) != 1 {
		t.Fatalf("want the one upgrade decision pending, got %v %v", pending, err)
	}
	store := ask.Answers{pending[0].Key(): answers.Entry{Question: pending[0].Text, Answer: said}}
	if err := ask.WriteAnswers(root, pending, store, time.Now()); err != nil {
		t.Fatal(err)
	}
}

// A recorded "install" consents, pinned to the digest in the decision.
// proved by: dropped the Pinned from the answered-install consent — the
// upgrade then installs whatever the release serves on an answer given
// about a specific digest.
func TestARecordedInstallAnswerInstallsThatDigest(t *testing.T) {
	root := governedRepo(t)
	s, _ := noTerminal(root)
	s.selfUpgradeConsent(false, "")(testPending)
	answer(t, root, "install")

	s, out := noTerminal(root)
	got := s.selfUpgradeConsent(false, "")(testPending)
	if !got.Install || got.Pinned != testSum {
		t.Fatalf("a recorded install must consent pinned to the decision's digest: %+v\n%s", got, out)
	}
	if !strings.Contains(out.String(), "installing on the answer recorded") {
		t.Errorf("the install must say what it acted on:\n%s", out)
	}
}

// "skip" — or any answer that is not install — installs nothing.
// proved by: treated any recorded answer as consent — "skip" then installs.
func TestARecordedSkipInstallsNothing(t *testing.T) {
	root := governedRepo(t)
	s, _ := noTerminal(root)
	s.selfUpgradeConsent(false, "")(testPending)
	answer(t, root, "skip")

	s, out := noTerminal(root)
	if got := s.selfUpgradeConsent(false, "")(testPending); got.Install {
		t.Fatalf("a recorded skip must not consent: %+v", got)
	}
	if !strings.Contains(out.String(), `is "skip", not install`) {
		t.Errorf("the refusal must name the answer:\n%s", out)
	}
}

// The decision is pinned to the release it was raised for: an install
// answer for 1.1.0 is not consent to 1.2.0, which gets its own decision.
func TestAnAnswerForOneReleaseIsNotConsentToTheNext(t *testing.T) {
	root := governedRepo(t)
	s, _ := noTerminal(root)
	s.selfUpgradeConsent(false, "")(testPending)
	answer(t, root, "install")

	next := releases.NewPending("1.0.0", "1.2.0", "procoder-test", testPending.URL, strings.Repeat("cd", 32))
	s, out := noTerminal(root)
	if got := s.selfUpgradeConsent(false, "")(next); got.Install {
		t.Fatalf("an answer about 1.1.0 must not install 1.2.0: %+v", got)
	}
	if !strings.Contains(out.String(), "recorded as a decision") {
		t.Errorf("1.2.0 must be raised as its own decision:\n%s", out)
	}
}

// --confirm is shape-checked before anything is asked of GitHub.
func TestConfirmArgumentsAreChecked(t *testing.T) {
	for _, argv := range [][]string{
		{"self-upgrade", "--confirm"},
		{"self-upgrade", "--confirm", "not-a-digest"},
		{"self-upgrade", "--confirm", strings.Repeat("a", 63)},
	} {
		s, out := noTerminal(t.TempDir())
		if code := run(argv, s); code != 2 || !strings.Contains(out.String(), "--confirm wants") {
			t.Errorf("%v: exit %d, %s", argv, code, out)
		}
	}
}
