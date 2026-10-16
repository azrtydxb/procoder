package ask

import (
	"fmt"
	"os"
	"path"
	"strings"

	"procoder/internal/answers"
	"procoder/internal/store"
)

// Decision is one recorded decision and what, if anything, a person
// answered to it — read through the same key `procoder ask` files the
// answer under, so an answer recorded there is the answer found here.
type Decision struct {
	Heading  string
	Answer   string
	Answered bool
}

// decisionsRel is the decisions file, repo-relative.
var decisionsRel = path.Join(Dir, DecisionsFile)

// FindDecision returns the first recorded decision whose heading starts
// with prefix, and the answer recorded against it.
//
// An unreadable decisions or answers file is an error, never "not found":
// a caller that acts on an answer must not mistake a file it could not read
// for a question nobody raised, and then raise it twice.
func FindDecision(root, prefix string) (Decision, bool, error) {
	if _, err := store.ReadFile(root, decisionsRel); err != nil && !os.IsNotExist(err) {
		return Decision{}, false, err
	}
	// The notes are about a file with no decision heading in it — which,
	// for a heading this caller is looking for, is simply "not raised".
	qs, _ := decisionQuestions(root)
	for _, q := range qs {
		heading, _, _ := strings.Cut(q.Text, "\n")
		if !strings.HasPrefix(heading, prefix) {
			continue
		}
		recorded, err := answers.Load(root)
		if err != nil {
			return Decision{}, false, err
		}
		d := Decision{Heading: heading}
		// The stable key first, the legacy one second — the order Settled
		// reads them in, so the two can never disagree about whether this
		// decision has an answer.
		for _, key := range []string{q.Key(), q.LegacyKey()} {
			if e, ok := recorded[key]; ok {
				d.Answer, d.Answered = e.Answer, true
				break
			}
		}
		return d, true, nil
	}
	return Decision{}, false, nil
}

// RaiseDecision appends a decision — heading, then body — unless one whose
// heading starts with prefix is already recorded. It reports whether it
// wrote. The read and the write happen under the file's lock, so two runs at
// once cannot both decide the heading is missing and record it twice.
//
// This is the one place procoder WRITES the decisions file, and it is a
// deliberate exception to DecisionsFile's "written by the agent" rule: the
// decision is `procoder self-upgrade`'s own — whether to replace the binary
// running it — and nobody else can phrase it with the digest in it. It
// writes a question and never an answer, so it decides nothing (#317).
func RaiseDecision(root, prefix, heading, body string) (bool, error) {
	if !strings.HasPrefix(heading, prefix) {
		return false, fmt.Errorf("decision heading %q does not start with its own key %q", heading, prefix)
	}
	release, err := store.Lock(root, decisionsRel)
	if err != nil {
		return false, err
	}
	defer release()
	raw, err := store.ReadFile(root, decisionsRel)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if h := strings.TrimPrefix(line, "## "); h != line && strings.HasPrefix(strings.TrimSpace(h), prefix) {
			return false, nil
		}
	}
	text := string(raw)
	if strings.TrimSpace(text) == "" {
		text = "# Decisions waiting on a human\n"
	}
	text = strings.TrimRight(text, "\n") + "\n\n## " + heading + "\n\n" + strings.TrimSpace(body) + "\n"
	return true, store.WriteFile(root, decisionsRel, []byte(text), 0o644)
}
