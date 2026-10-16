package ask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"procoder/internal/answers"
)

const (
	raisePrefix  = "Upgrade procoder 1.0.0 → 1.1.0 "
	raiseHeading = raisePrefix + "(sha256 abc)?"
	raiseBody    = "- install: verified against abc\n- skip: stay on 1.0.0"
)

// A raised decision is written once, beside what is already there, and a
// second raise of the same key writes nothing.
// proved by: dropped the existing-heading scan in RaiseDecision — the
// second run then appends a duplicate heading.
func TestRaiseDecisionWritesOnce(t *testing.T) {
	root := t.TempDir()
	writeDecisions(t, root, "# Decisions waiting on a human\n\n## Something else?\n\n- a\n- b\n")
	for i, want := range []bool{true, false} {
		added, err := RaiseDecision(root, raisePrefix, raiseHeading, raiseBody)
		if err != nil {
			t.Fatal(err)
		}
		if added != want {
			t.Errorf("raise %d: added = %v, want %v", i+1, added, want)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Dir), DecisionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), "## "+raisePrefix); n != 1 {
		t.Errorf("the heading must appear once, got %d:\n%s", n, raw)
	}
	if !strings.Contains(string(raw), "## Something else?") {
		t.Errorf("what was already recorded must survive:\n%s", raw)
	}
	// The raised decision is collected like any other, options included.
	qs, _ := decisionQuestions(root)
	if len(qs) != 2 || !strings.Contains(qs[1].Text, "- skip: stay on 1.0.0") {
		t.Errorf("the raised decision must be collected with its options: %+v", qs)
	}
}

// An absent file is created with the heading the hand-written ones carry.
func TestRaiseDecisionCreatesTheFile(t *testing.T) {
	root := t.TempDir()
	if added, err := RaiseDecision(root, raisePrefix, raiseHeading, raiseBody); err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(Dir), DecisionsFile))
	if !strings.HasPrefix(string(raw), "# Decisions waiting on a human\n\n## "+raiseHeading+"\n\n- install") {
		t.Errorf("unexpected file:\n%s", raw)
	}
}

// The answer is found under the key `procoder ask` files it under — an
// answer recorded through the ordinary route is the answer read here.
// proved by: looked the answer up by the heading's key alone (without the
// options) — the recorded answer is then never found.
func TestFindDecisionReadsTheAnswerAskRecorded(t *testing.T) {
	root := t.TempDir()
	if _, err := RaiseDecision(root, raisePrefix, raiseHeading, raiseBody); err != nil {
		t.Fatal(err)
	}
	d, found, err := FindDecision(root, raisePrefix)
	if err != nil || !found || d.Answered || d.Heading != raiseHeading {
		t.Fatalf("before an answer: %+v found=%v err=%v", d, found, err)
	}
	qs, _ := decisionQuestions(root)
	store := Answers{qs[0].Key(): answers.Entry{Question: qs[0].Text, Answer: "install"}}
	if err := WriteAnswers(root, qs, store, time.Now()); err != nil {
		t.Fatal(err)
	}
	d, found, err = FindDecision(root, raisePrefix)
	if err != nil || !found || !d.Answered || d.Answer != "install" {
		t.Fatalf("after an answer: %+v found=%v err=%v", d, found, err)
	}
	if _, found, _ := FindDecision(root, "Upgrade procoder 1.0.0 → 1.2.0 "); found {
		t.Error("a different version's decision must not be found")
	}
}
