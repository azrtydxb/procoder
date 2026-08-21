package ask

import (
	"os"
	"path/filepath"
	"procoder/internal/answers"
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
	qs, _ := Collect(root)
	if err := WriteAnswers(root, qs, Answers{qs[0].Key(): answers.Entry{Question: qs[0].Text, Answer: "intentional, it is a fixture"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := GateFindings(root); len(got) != 0 {
		t.Errorf("an answered question is not a finding: %+v", got)
	}
}

// The gate must not claim silence it has not earned. This test was vacuous
// when it was written: it asserted the gate reports NOTHING on the error
// path, which was the shipped behaviour, so the mutation its own comment
// named could never turn it red. A pre-PR review applied that mutation and
// watched it stay green.
// proved by: returning nil on the error path again — the gate then says
// there is nothing to decide because it could not look, and this fails.
func TestAnUnreadableRecordIsNotAnEmptyGateVerdict(t *testing.T) {
	root := repo(t, "anything?")
	if err := os.MkdirAll(Path(root, "answers.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := GateFindings(root)
	if len(got) != 1 {
		t.Fatalf("a record that cannot be read is one finding, not silence: %+v", got)
	}
	if !strings.Contains(got[0].Message, "NOT collected") {
		t.Errorf("and it says so: %+v", got[0])
	}
	if got[0].Blocking {
		t.Errorf("an unreadable record is a report, not a refusal: %+v", got[0])
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
