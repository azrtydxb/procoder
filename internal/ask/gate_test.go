package ask

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// C-08: the policy decides whether a question stops a commit. Report is the
// default because a question is a request for judgement, not a defect —
// blocking on one stops work the person who could unblock it may be asleep
// for.
// proved by: made the findings blocking regardless of config — the default
// repository then cannot commit while any question is open.
func TestTheAskPolicyDecidesWhetherQuestionsBlock(t *testing.T) {
	root := repo(t, "is this flag intentional?")
	findings := GateFindings(root)
	if len(findings) == 0 {
		t.Fatal("an open question must reach the gate")
	}
	for _, f := range findings {
		if f.Blocking {
			t.Errorf("report is the default: %+v", f)
		}
	}

	if err := os.WriteFile(filepath.Join(root, ".procoder", "config.toml"),
		[]byte("[ask]\npolicy = \"block\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := GateFindings(root)
	if len(blocked) == 0 {
		t.Fatal("the questions are still there under block")
	}
	for _, f := range blocked {
		if !f.Blocking {
			t.Errorf("block means block: %+v", f)
		}
	}

	// Answered: nothing left to report either way.
	qs := Collect(root)
	if err := WriteAnswers(root, qs, Answers{qs[0].Key(): "intentional, it is a fixture"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := GateFindings(root); len(got) != 0 {
		t.Errorf("an answered question is not a finding: %+v", got)
	}
}

// The gate must not claim silence it has not earned: a collection that could
// not run reports nothing rather than "no questions".
// proved by: returned a clean verdict on the error path — the gate then says
// there is nothing to decide because it could not look.
func TestAnUnreadableRecordIsNotAnEmptyGateVerdict(t *testing.T) {
	root := repo(t, "anything?")
	if err := os.MkdirAll(Path(root, "answers.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := GateFindings(root); len(got) != 0 {
		t.Errorf("nothing is reported when nothing could be read: %+v", got)
	}
	// and the command itself refuses loudly, which is where the user learns
	out, lines := collect(t)
	if code := Run(root, nil, out); code != 2 {
		t.Fatalf("the command says so: exit %d %v", code, *lines)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "NOT read") {
		t.Errorf("and names what it could not read: %v", *lines)
	}
}
