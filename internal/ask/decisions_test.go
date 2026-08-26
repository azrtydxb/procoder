package ask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeDecisions(t *testing.T, root, body string) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, DecisionsFile)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// S-1: a decision the agent wrote down is collected alongside the four
// sources the repository computes.
//
// proved by: the `decisionQuestions` call removed from Collect (the
// decision vanishes, want 2 got 0).
func TestRecordedDecisionsAreCollected(t *testing.T) {
	root := repo(t)
	writeDecisions(t, root, `## Merge #187 before or after #181?

- before: the gate fix lands first
- after: one release

## File the kubeconform flake?
`)
	qs, notes := Collect(root)
	var got []Question
	for _, q := range qs {
		if q.Source == "decision" {
			got = append(got, q)
		}
	}
	if len(got) != 2 {
		t.Fatalf("want 2 decisions, got %d (%+v) notes=%v", len(got), got, notes)
	}
	if !strings.Contains(got[0].Text, "Merge #187") {
		t.Errorf("the decision's heading was lost: %q", got[0].Text)
	}
	// The options are part of the question. A decision presented without
	// them is the prose question this whole change exists to replace.
	if !strings.Contains(got[0].Text, "before: the gate fix lands first") {
		t.Errorf("the decision's options were dropped: %q", got[0].Text)
	}
}

// S-1's quiet half: no decisions file is the overwhelmingly common case.
// It is not a finding, and a note in front of every user who never writes
// one would be noise that trains people to ignore notes.
//
// proved by: the os.IsNotExist branch made to return a note (want no
// notes, got one).
func TestNoDecisionsFileIsSilent(t *testing.T) {
	root := repo(t)
	qs, notes := Collect(root)
	for _, q := range qs {
		if q.Source == "decision" {
			t.Fatalf("a decision appeared with no file: %+v", q)
		}
	}
	for _, n := range notes {
		if strings.Contains(n, DecisionsFile) {
			t.Fatalf("a missing decisions file produced a note: %q", n)
		}
	}
}

// No silent green: content nobody can parse must not read as no content.
// The decisions are on disk and would otherwise never be asked.
//
// proved by: `return nil, []string{...}` → `return nil, nil` for the
// no-heading case (want a note, got none).
func TestAMalformedDecisionsFileSaysSo(t *testing.T) {
	root := repo(t)
	writeDecisions(t, root, "I need to decide whether to merge first.\n")
	qs, notes := Collect(root)
	for _, q := range qs {
		if q.Source == "decision" {
			t.Fatalf("an unparseable file produced a question: %+v", q)
		}
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, DecisionsFile) {
			found = true
		}
	}
	if !found {
		t.Fatalf("content that could not be read was passed over in silence: %v", notes)
	}
}

// S-2, and the constraint the tool rests on: P-CONTROL. The binary prints
// and the agent writes. A decisions queue the binary wrote to would be the
// most natural thing in the world to add and would break the rule.
//
// proved by: an os.WriteFile added to decisionQuestions that rewrites the
// file it just read (the digest changes, and the test names the file).
func TestCollectingDecisionsWritesNothing(t *testing.T) {
	root := repo(t)
	body := "## Merge #187 before or after #181?\n\n- before\n- after\n"
	p := writeDecisions(t, root, body)
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}

	Collect(root)

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("the decisions file did not survive being read: %v", err)
	}
	if string(raw) != body {
		t.Fatalf("procoder rewrote a file the agent owns (P-CONTROL):\nwant %q\ngot  %q", body, raw)
	}
	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("the decisions file was touched: %s → %s", before.ModTime(), after.ModTime())
	}
}

// S-4: an edited decision is a different decision, and must be asked
// again. The key hashes what was asked, so this falls out of the existing
// machinery rather than being re-invented — the test pins that it does.
//
// proved by: Question.Key() made to ignore Text (the two keys collide, and
// an edited decision reads as already answered).
func TestAnEditedDecisionIsANewQuestion(t *testing.T) {
	root := repo(t)
	writeDecisions(t, root, "## Merge before or after?\n")
	first, _ := Collect(root)

	writeDecisions(t, root, "## Merge before, after, or not at all?\n")
	second, _ := Collect(root)

	keyOf := func(qs []Question) string {
		t.Helper()
		for _, q := range qs {
			if q.Source == "decision" {
				return q.Key()
			}
		}
		t.Fatal("no decision collected")
		return ""
	}
	if keyOf(first) == keyOf(second) {
		t.Fatal("an edited decision kept its old key, so a stale answer would silence it")
	}
}

// S-3: an outstanding decision is visible where the verdict is read.
// Without this the queue is decoration — the agent records a decision,
// nothing reports it, and the run presents as having nothing outstanding.
//
// proved by: the `decisionQuestions` call removed from Collect (the gate
// reports nothing, want a finding).
func TestTheGateReportsAnOutstandingDecision(t *testing.T) {
	root := repo(t)
	writeDecisions(t, root, "## Merge #187 before or after #181?\n\n- before\n- after\n")

	found := false
	for _, f := range GateFindings(root) {
		if strings.Contains(f.Message, "waiting on a human") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a recorded decision was invisible to the gate: %+v", GateFindings(root))
	}
}

// The paired half, so the test above cannot pass by the gate always
// reporting something: with no decision recorded, this fixture is quiet.
//
// proved by: `if len(pending) == 0 { return nil }` removed from
// GateFindings (a finding appears with nothing pending).
func TestTheGateIsQuietWithNoDecision(t *testing.T) {
	root := repo(t)
	for _, f := range GateFindings(root) {
		if strings.Contains(f.Message, "waiting on a human") {
			t.Fatalf("the gate reported a pending question with none recorded: %+v", f)
		}
	}
}

// procoder must not write a file that procoder's own gate rejects.
//
// The decisions queue made this reachable for the first time: a decision
// carries its options as a markdown list, and a list beginning on the line
// straight after a paragraph makes a formatter reflow what follows —
// pulling the `Answer:` line into the list as a continuation, and stripping
// the trailing space from an empty `Answer: `. The result was `procoder
// ask` writing QA.md and the very next `procoder check` blocking the commit
// as unformatted. It blocked this sprint's own commit, which is how it was
// found.
//
// Asserted as a property of the bytes rather than by shelling out to
// prettier, which is not installed everywhere and would make this test's
// verdict depend on the machine.
//
// proved by: the `"\n\n"` after the question text reverted to `"\n"`, and
// separately the TrimRight removed — each leaves a line the formatter
// would rewrite.
func TestTheGeneratedFilesAreFormatStable(t *testing.T) {
	root := repo(t)
	writeDecisions(t, root, "## Ship now or wait?\n\n- now: small and tested\n- wait: keeps the release scoped\n")
	qs, _ := Collect(root)
	if err := WriteQuestions(root, qs, time.Unix(0, 0)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(Path(root, QuestionsFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")

	for i, line := range lines {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("line %d has trailing whitespace, which a formatter strips: %q", i+1, line)
		}
		// The failure that started this: `Answer:` sitting directly under a
		// list item is read as part of it and gets indented.
		if strings.HasPrefix(line, "Answer:") && i > 0 {
			if prev := strings.TrimSpace(lines[i-1]); prev != "" {
				t.Errorf("line %d: `Answer:` follows %q with no blank line — a formatter will indent it into that list", i+1, prev)
			}
		}
		// The other half: a list opening immediately after a paragraph.
		if strings.HasPrefix(strings.TrimSpace(line), "- ") && i > 0 {
			prev := strings.TrimSpace(lines[i-1])
			if prev != "" && !strings.HasPrefix(prev, "- ") {
				t.Errorf("line %d: a list opens directly under %q with no blank line", i+1, prev)
			}
		}
	}
}
