package answers

import (
	"strings"
	"testing"
)

// A question re-wrapped across lines is the same question. The legacy Key
// hashes the text as written, so a reformatted section changed the hash and
// orphaned every answer recorded under it — the answered decision came back
// on the next run. KeyStable hashes the words, not the line breaks.
//
// proved by: KeyStable made to hash the raw text — the reflow case fails.
func TestAReflowedQuestionKeepsItsKey(t *testing.T) {
	oneliner := "Issue #107 is not reachable — close it, or ship the comment too?"
	// The same question the way a formatter rewraps it across lines:
	reflowed := "Issue #107 is not reachable — close it, or\nship the comment too?"

	if KeyStable("decision", "decisions.md", oneliner) != KeyStable("decision", "decisions.md", reflowed) {
		t.Error("a reflowed question changed its key, so an answer recorded under the old line breaks would be re-asked")
	}
	if Key("decision", "decisions.md", oneliner) == Key("decision", "decisions.md", reflowed) {
		t.Log("note: legacy Key happens to agree here — the reflow cases that disagree are the ones this fix exists for")
	}

	// Double spaces and trailing whitespace are line breaks in disguise:
	padded := "  Issue #107 is not reachable — close it,   or\nship the comment too? "
	if KeyStable("decision", "decisions.md", oneliner) != KeyStable("decision", "decisions.md", padded) {
		t.Error("whitespace-only differences changed the stable key")
	}
}

// A reworded question is a new question: the answer belonged to different
// words. This is the behaviour Key documented and the test pins, so the
// stability fix is not read as a blanket "answers never expire".
//
// proved by: KeyStable made to ignore the text entirely — both keys collide.
func TestARewordedQuestionGetsANewKey(t *testing.T) {
	a := KeyStable("decision", "decisions.md", "Merge before or after?")
	b := KeyStable("decision", "decisions.md", "Merge before, after, or not at all?")
	if a == b {
		t.Fatal("a reworded question kept its key, so a stale answer would silence it")
	}

	// The same sentence from two domains is still two questions:
	if KeyStable("spec", "s.md", a) == KeyStable("decision", "d.md", a) {
		t.Error("source and origin must stay in the key")
	}
}

// A store written before KeyStable was introduced holds its answers under
// the legacy key. Settled must find them, or the upgrade re-asks every
// decision that was already settled.
//
// proved by: the legacy lookup removed from Settled — the case fails.
func TestSettledFindsAnswersRecordedUnderTheLegacyKey(t *testing.T) {
	// Multi-line on purpose: that is exactly the shape where the two
	// generations of key disagree.
	body := "Issue #107 is not reachable — close it, or ship the comment too?\n\n" +
		"- close it, with the evidence\n- ship the comment as well"

	store := Store{
		Key("decision", "decisions.md", body): {Question: body, Answer: "close it, with the evidence"},
	}
	if !store.Settled("decision", "decisions.md", body) {
		t.Fatal("an answer recorded under the legacy key did not read as settled")
	}

	// And an answer recorded under the STABLE key settles the question too:
	stable := Store{
		KeyStable("decision", "decisions.md", body): {Question: body, Answer: "ship the comment"},
	}
	if !stable.Settled("decision", "decisions.md", body) {
		t.Fatal("an answer recorded under the stable key did not read as settled")
	}

	// A question nobody answered is open under either generation:
	if store.Settled("decision", "decisions.md", "A different question entirely?") {
		t.Error("an unrelated question read as settled")
	}
}

// Key and KeyStable agree on the text neither generation needs to repair —
// a single-line, already-canonical question — so a store full of single-line
// answers is readable without the fallback ever firing.
func TestTheTwoGenerationsOfKeyAgreeOnCanonicalText(t *testing.T) {
	text := "Which version of the Agent Plugins specification is current?"
	if Key("spec", "marketplace-strategy", text) != KeyStable("spec", "marketplace-strategy", text) {
		t.Error("canonical text must key identically in both generations")
	}
	if strings.Count(text, "\n") != 0 {
		t.Error("the test premise is a single-line question")
	}
}
