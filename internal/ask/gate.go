package ask

import (
	"fmt"
	"path"

	"procoder/internal/answers"
	"procoder/internal/config"
	"procoder/internal/gitx"
)

// GateFindings is the gate's half of this loop: what a human still has to
// decide, reported where the rest of the verdict is read.
//
// Report by default (D-4). A pending question is a request for judgement,
// not a defect, and failing a commit on one stops work the person who could
// unblock it may not be awake for. `[ask] policy = "block"` is there for a
// repository that wants the harder rule.
//
// A collection that cannot run says nothing rather than guessing: the
// questions are gathered from five sources — four the repository computes,
// plus the decisions the agent recorded — and a gate that reported "no
// questions" because it could not read the answers file would be claiming
// something it does not know.
func GateFindings(root string) []gitx.Finding {
	pending, err := Pending(root)
	if err != nil {
		// A record that cannot be read is not an empty one. Collapsing the
		// two would make the gate say there is nothing to decide because it
		// could not look — the exact silence this domain exists to end.
		return []gitx.Finding{{
			File:    path.Join(Dir, answers.File),
			Message: "questions NOT collected — " + err.Error() + " (ask)",
		}}
	}
	if len(pending) == 0 {
		return nil
	}
	block := config.Load(root).AskBlock
	qa := path.Join(Dir, QuestionsFile)
	out := []gitx.Finding{{
		File:     qa,
		Blocking: block,
		Message: fmt.Sprintf("%d question(s) waiting on a human — run `procoder ask` (ask)",
			len(pending)),
	}}
	for _, q := range pending {
		out = append(out, gitx.Finding{
			File:     q.Origin,
			Blocking: block,
			Message:  q.Text + " (ask)",
		})
	}
	return out
}
