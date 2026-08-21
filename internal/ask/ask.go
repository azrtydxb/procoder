// Package ask is the half of the loop that was missing: every domain here
// already knows what it cannot decide, and until now those questions reached
// the AI coder as prose, which answered them itself. An invented answer is
// indistinguishable from a decision once it is written down.
//
// ask collects the questions, puts them to a person, and records what they
// said where the coder and the gate can both read it. It decides nothing.
package ask

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"procoder/internal/answers"
	"procoder/internal/docs"
	"procoder/internal/gitx"
	"procoder/internal/lint"
	"procoder/internal/security"
	"procoder/internal/spec"
)

// Dir holds the two files, beside the other state this repository keeps.
const Dir = ".procoder/ask"

// Question is one thing a human has to decide. Origin is where it came from —
// a spec name, a file path — so an answer can be traced back to what asked.
type Question struct {
	Source string // spec | docs | security | lint
	Origin string
	Text   string
}

// Key identifies the question across runs — see answers.Key for why it
// hashes what was asked rather than where it appeared.
func (q Question) Key() string { return answers.Key(q.Source, q.Origin, q.Text) }

// Label is the one-line heading a question is filed under.
func (q Question) Label() string {
	origin := q.Origin
	if origin == "" {
		origin = "(no origin)"
	}
	return fmt.Sprintf("[%s] %s", q.Source, origin)
}

// Collect gathers every question the repository can currently pose, in a
// stable order so two runs produce the same file. Domains that cannot answer
// contribute nothing rather than failing the collection: a lint tool that is
// not installed is not a question, and losing the other three because of it
// would be the worst outcome.
func Collect(root string) []Question {
	changed, err := gitx.ChangedFiles(root)
	if err != nil {
		changed = nil
	}
	var qs []Question
	qs = append(qs, specQuestions(root)...)
	qs = append(qs, findingQuestions("docs", docs.Obligation(root, changed, "", false),
		"is this documentation gap real, or does the change genuinely need no doc?")...)
	qs = append(qs, findingQuestions("security", security.SecretsChangedFiles(root, changed),
		"is this a real credential, or a test value that only looks like one?")...)
	qs = append(qs, findingQuestions("lint", lint.Files(root, changed, false),
		"is this finding worth fixing here, or a false positive to be explained?")...)
	sort.SliceStable(qs, func(i, j int) bool {
		if qs[i].Source != qs[j].Source {
			return qs[i].Source < qs[j].Source
		}
		return qs[i].Origin+qs[i].Text < qs[j].Origin+qs[j].Text
	})
	return qs
}

// specQuestions reads what each spec still has undecided. It takes the WHOLE
// Open questions section, not the `OPEN:` prefix alone: spec.Check blocks on
// anything left in that section whatever it is called, and a collector that
// only saw the prefix would leave a human unable to answer what the gate is
// refusing on.
func specQuestions(root string) []Question {
	var qs []Question
	for _, path := range spec.Files(root) {
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		for _, line := range spec.OpenQuestions(path) {
			qs = append(qs, Question{Source: "spec", Origin: name, Text: line})
		}
	}
	return qs
}

// findingQuestions turns a domain's findings into questions. The finding's
// message is the evidence; the question is the same for every finding of that
// domain, because what a human adds is judgement, not detail.
//
// A security finding's message is deliberately NOT trusted to be free of the
// value it flagged: only the location is carried through. The question is
// whether the flag is real, and answering it does not need the credential.
func findingQuestions(source string, findings []gitx.Finding, question string) []Question {
	var qs []Question
	for _, f := range findings {
		where := filepath.ToSlash(f.File)
		if where == "" {
			// A finding about the repository rather than a file still needs
			// an origin a reader can act on.
			where = "(repository)"
		}
		if f.Line > 0 {
			where = fmt.Sprintf("%s:%d", where, f.Line)
		}
		text := question
		if source != "security" {
			text = f.Message + " — " + question
		}
		qs = append(qs, Question{Source: source, Origin: where, Text: text})
	}
	return qs
}
