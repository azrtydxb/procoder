package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"procoder/internal/ask"
	"strings"
	"testing"
)

// askRepo is a bare directory with a .procoder/ask/ — no git, because
// nothing here needs it and a Stop hook that required a repository would
// be one more way for a session to fail to end.
func askRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".procoder", "ask"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func recordDecision(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".procoder", "ask", "decisions.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runStop feeds a Stop payload and returns the exit code and whatever the
// hook wrote as its reason.
func runStop(t *testing.T, root, message string) (int, string) {
	t.Helper()
	payload, err := json.Marshal(stopPayload{Cwd: root, LastAssistantMessage: message})
	if err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	prev := Stderr
	Stderr = &errOut
	defer func() { Stderr = prev }()
	return Stop(bytes.NewReader(payload), root), errOut.String()
}

// S-1, S-2: the failure this exists for. The rule shipped in 3.2.0 and was
// broken the same day by the agent that wrote it — three decisions in the
// last paragraph of a triage report, work continuing underneath them.
//
// proved by: `decisionInProse` made to return false — the turn ends and
// the decision stays buried.
func TestAProseDecisionDoesNotEndTheTurn(t *testing.T) {
	root := askRepo(t)
	code, reason := runStop(t, root, `Triage done. Twelve issues reviewed.

Decisions I need from you:
1. Should I rescope the two that overlap?
2. Do you want me to start on the merge-conflict one?`)
	if code != 2 {
		t.Fatalf("exit %d, want 2 — the turn ended with the decision buried\n%s", code, reason)
	}
	// The reason has to be actionable, or it is a block nobody can satisfy.
	for _, want := range []string{"decisions.md", "structured question tool", "not fire twice"} {
		if !strings.Contains(reason, want) {
			t.Errorf("the reason does not contain %q:\n%s", want, reason)
		}
	}
}

// S-3: the agent did the thing. Being told about it anyway is how a check
// stops being read.
//
// proved by: the `len(pending) > 0` branch removed — a correctly recorded
// decision blocks anyway.
func TestARecordedDecisionIsSilent(t *testing.T) {
	root := askRepo(t)
	recordDecision(t, root, "## Should the two overlapping issues be rescoped?\n\n- yes\n- no\n")
	code, reason := runStop(t, root, "Triage done. Should I start on the merge-conflict one?")
	if code != 0 {
		t.Fatalf("exit %d with a decision recorded, want 0\n%s", code, reason)
	}
}

// S-4, and the criterion that matters most: a false block fires on
// ordinary work, at the end of every turn, and a check that interrupts
// correct work is one people route around — the failure behind #172 and
// #185.
//
// The fixtures are real messages from the session that built this.
//
// proved by: askPhrases replaced with `\?` — every one of these blocks.
func TestOrdinaryReportsDoNotBlock(t *testing.T) {
	for name, msg := range map[string]string{
		"a status summary":         "v3.2.1 published and verified. All five binaries plus SHA256SUMS, checksum matches, the binary reports 3.2.1.",
		"a rhetorical question":    `Why did this fail? Because the token is an app installation token and there is no user behind it. Fixed by making the exclusion configuration.`,
		"a finding report":         "The gate caught a real defect: a decision carries its options as a markdown list, and a list opening directly under a paragraph makes a formatter reflow what follows.",
		"work completed":           "Both PRs merged. main is at 7b3bd37, zero open PRs, all seven issues closed.",
		"a question in the middle": "I asked whether you want me to keep it, and you said yes, so it stays. The suite is green and the gate is clean.",
	} {
		root := askRepo(t)
		if code, reason := runStop(t, root, msg); code != 0 {
			t.Errorf("%s blocked the turn (exit %d):\n%s\n--- message:\n%s", name, code, reason, msg)
		}
	}
}

// S-5: a block the agent cannot satisfy would loop the session, which is
// worse than the failure being prevented.
//
// proved by: the rememberBlocked call removed — the second stop blocks
// again and the session cannot end.
func TestTheSameMessageNeverBlocksTwice(t *testing.T) {
	root := askRepo(t)
	msg := "Done. Should I cut the release now?"
	if code, _ := runStop(t, root, msg); code != 2 {
		t.Fatalf("the first stop must block, got %d", code)
	}
	if code, reason := runStop(t, root, msg); code != 0 {
		t.Fatalf("the same message blocked twice (exit %d) — the session cannot end\n%s", code, reason)
	}
}

// S-6: a blocked turn must not also lose its handoff note.
//
// proved by: the block moved above the note write — the note is missing
// on the blocking path.
func TestTheHandoffNoteSurvivesABlock(t *testing.T) {
	root := askRepo(t)
	if code, _ := runStop(t, root, "Done. Should I tag it?"); code != 2 {
		t.Fatalf("the fixture did not block, so this proves nothing (exit %d)", code)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(StateDir), HandoffFile)); err != nil {
		t.Fatalf("the handoff note was lost on the blocking path: %v", err)
	}
}

// An absent field is not evidence of anything — an older host, another
// host, or a malformed event must not end up blocking.
//
// The empty-message guard in unaskedDecision is an early return, not the
// protection — an empty string matches no ask phrase anyway. Verified:
// removing it does not fail this test. What this pins is the behaviour, so
// a future detector that treats absence as something fails here.
func TestNoMessageIsNotEvidence(t *testing.T) {
	root := askRepo(t)
	if code, reason := runStop(t, root, ""); code != 0 {
		t.Fatalf("a payload with no message blocked (exit %d)\n%s", code, reason)
	}
}

// The distinction that cost a false positive: an interrogative phrase is
// an ask only when it is actually asked. Narration about a decision
// already taken uses the same words.
//
// proved by: the `strings.Contains(s, "?")` test dropped from the
// interrogative branch — the narration blocks, which is what it did before
// this test existed.
func TestNarrationAboutAPastDecisionIsNotAnAsk(t *testing.T) {
	for name, msg := range map[string]string{
		"past tense":  "I asked whether you want me to keep it, and you said yes, so it stays.",
		"reported":    "You told me which one you would prefer, so that is what shipped.",
		"conditional": "If you had said you want me to hold it, I would have.",
	} {
		root := askRepo(t)
		if code, reason := runStop(t, root, msg); code != 0 {
			t.Errorf("%s was read as an ask (exit %d):\n%s", name, code, reason)
		}
	}
}

// And a decision handed over without a question mark is still a decision.
// "Say the word and I'll tag it" puts the choice to somebody exactly as
// much as "shall I tag it?" does.
//
// proved by: the deferring branch removed — these end the turn with the
// decision buried.
func TestADecisionHandedOverWithoutAQuestionMarkStillBlocks(t *testing.T) {
	for name, msg := range map[string]string{
		"say the word": "Everything is verified and ready. Say the word and I'll tag it.",
		"your call":    "Both options work. Your call.",
		"up to you":    "I would keep it open, but it is up to you.",
	} {
		root := askRepo(t)
		if code, _ := runStop(t, root, msg); code != 2 {
			t.Errorf("%s did not block (exit %d) — the decision ends the turn unasked", name, code)
		}
	}
}

// Unknown is not none. A decisions file that exists and cannot be read, or
// holds content in a shape procoder does not recognise, means whether a
// decision was recorded is UNKNOWN — and blocking a turn on not knowing is
// a block nobody can act on, which is the thing this hook exists to
// prevent rather than to cause.
//
// The spec said so in its edge cases and the first implementation did the
// opposite: PendingDecisions discarded the notes decisionQuestions
// produces for exactly these cases, so unknown arrived as none and the
// turn was blocked. Raised in review on #215; reproduced before fixing.
//
// proved by: the `len(notes) > 0` test dropped from unaskedDecision — the
// malformed file blocks again.
func TestAnUnreadableDecisionsFileDoesNotBlock(t *testing.T) {
	root := askRepo(t)
	recordDecision(t, root, "I need to decide whether to merge first.\n")
	if code, reason := runStop(t, root, "Done. Should I tag it now?"); code != 0 {
		t.Errorf("an unparseable decisions file blocked the turn (exit %d) — procoder cannot tell whether a decision was recorded:\n%s", code, reason)
	}
}

// And the neighbouring case, which is NOT unknown: a file that exists and
// is empty says plainly that nothing was recorded. That is a fact, and the
// turn should still block.
//
// Written after the first version of the test above lumped the two
// together and failed — an empty file is not an unreadable one.
//
// proved by: the `strings.TrimSpace(string(raw)) != ""` test in
// decisionQuestions inverted, so an empty file produces a note — the turn
// then stops blocking and a buried decision gets through.
func TestAnEmptyDecisionsFileStillBlocks(t *testing.T) {
	root := askRepo(t)
	recordDecision(t, root, "   \n")
	if code, _ := runStop(t, root, "Done. Should I tag it now?"); code != 2 {
		t.Fatalf("an empty decisions file did not block (exit %d) — empty means nothing was recorded", code)
	}
}

// The defect this fixes, reproduced: a decision recorded in an EARLIER
// turn used to silence the hook for every later turn, whatever they
// buried. Measured mid-session on this repository — six pending decisions
// and the hook mute since the first of them.
//
// proved by: restore `len(pending) > 0` as an unconditional silencer in
// unaskedDecision — the second call stops blocking and this fails.
func TestAStaleBacklogDoesNotSilenceTheHook(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(ask.Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	decisions := filepath.Join(dir, ask.DecisionsFile)
	write := func(body string) {
		if err := os.WriteFile(decisions, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Turn one: the agent records a decision, so the hook stays quiet.
	write("## Ship it now or hold?\n\n- now: ...\n- hold: ...\n")
	if blocked, _ := unaskedDecision(root, "Should I ship it now, or hold?"); blocked {
		t.Fatal("blocked a turn that recorded its decision")
	}

	// Turn two: a DIFFERENT decision, in prose, and the file untouched.
	// The backlog from turn one must not excuse it.
	blocked, msg := unaskedDecision(root, "Do you want the cache dropped as well?")
	if !blocked {
		t.Fatal("a stale backlog silenced the hook — the whole defect")
	}
	if !strings.Contains(msg, "decisions.md") {
		t.Errorf("the refusal must name the file: %q", msg)
	}
}

// proved by: drop the `readErr != nil` arm in decisionsChanged so a
// missing digest reads as unchanged — the first stop in a fresh tree then
// blocks, and this fails.
func TestTheFirstStopInATreeDoesNotBlock(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(ask.Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ask.DecisionsFile),
		[]byte("## Something already here?\n\n- a: ...\n- b: ...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if blocked, _ := unaskedDecision(root, "Shall I go ahead?"); blocked {
		t.Error("blocked on the first stop, with nothing to compare against")
	}
}
