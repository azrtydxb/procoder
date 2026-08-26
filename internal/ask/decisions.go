package ask

import (
	"fmt"
	"os"
	"path/filepath"
	"procoder/internal/answers"
	"strings"
)

// DecisionsFile is where the agent records a decision it must not make.
//
// The other four sources are computed from the repository: a spec's open
// questions, a docs obligation, a flagged secret, a lint finding. A
// decision is not like them. It does not come from a finding — it comes
// from the work, when the next step forks and the fork is not the agent's
// to pick. Nothing computes that, so it has to be written down.
//
// Written by the AGENT, read by procoder. Never the other way round.
// P-CONTROL is the rule the whole tool rests on, and a `procoder ask
// --raise "..."` that wrote this file would read better on the command
// line and break it. See the spec's Decisions section.
const DecisionsFile = "decisions.md"

// decisionQuestions reads the decisions the agent has recorded.
//
// The format is deliberately the plainest thing that survives being
// hand-edited: a `## ` heading is one decision, and the lines under it are
// its options and context. Anything a person can read in the file they are
// already being asked to answer.
//
//	## Merge #187 before or after #181?
//	- before: the gate fix lands first, #181 rebases onto it
//	- after: one release, but #181 is written against the old gate
//
// A missing file is the overwhelmingly common case and contributes
// nothing. It is not a finding, and treating it as one would put a note in
// front of every user who never writes decisions.
func decisionQuestions(root string) ([]Question, []string) {
	path := Path(root, DecisionsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		// Unreadable is not the same as empty, and must not look like it.
		// A file that exists and could not be read means decisions may be
		// waiting that nobody will see.
		return nil, []string{fmt.Sprintf("%s NOT read (%v) — recorded decisions were not collected",
			filepath.ToSlash(filepath.Join(Dir, DecisionsFile)), err)}
	}

	var qs []Question
	var current string
	var body []string
	flush := func() {
		if current == "" {
			return
		}
		text := current
		if detail := strings.TrimSpace(strings.Join(body, "\n")); detail != "" {
			// A BLANK line between the heading and the options, not just a
			// newline. The options are a markdown list, and a list that
			// begins on the line straight after a paragraph makes a
			// formatter reflow everything around it — including the
			// `Answer:` line, which it pulls into the list as a
			// continuation. The file then fails procoder's own formatting
			// check, on every commit that follows a recorded decision.
			text = current + "\n\n" + detail
		}
		qs = append(qs, Question{Source: "decision", Origin: DecisionsFile, Text: text})
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if heading := strings.TrimPrefix(line, "## "); heading != line {
			flush()
			current, body = strings.TrimSpace(heading), nil
			continue
		}
		if current != "" {
			body = append(body, line)
		}
	}
	flush()

	if len(qs) == 0 && strings.TrimSpace(string(raw)) != "" {
		// Content with no `## ` heading: somebody wrote decisions in a
		// shape this cannot read. Silence here would be the worst answer —
		// the decisions are on disk and would never be asked.
		return nil, []string{fmt.Sprintf("%s has content but no `## ` decision heading — nothing was collected from it",
			filepath.ToSlash(filepath.Join(Dir, DecisionsFile)))}
	}
	return qs, nil
}

// PendingDecisions is the decisions the agent recorded and nobody has
// answered yet.
//
// Deliberately narrow. Pending() runs the whole collection — git, the lint
// pass, the secret scan — because it answers "what is outstanding
// anywhere". A Stop hook asking only "did the agent write a decision down"
// cannot afford that: it runs at the end of every turn, under a ten-second
// timeout, and a check that makes a session hang is worse than the rule it
// enforces.
// The notes are returned, not discarded. decisionQuestions produces one
// when the file exists and cannot be read, or holds content in a shape it
// does not recognise — cases where "no decisions were recorded" is not a
// fact, it is an absence of information.
//
// Dropping them made a caller unable to tell the two apart, and the Stop
// hook then blocked a turn because it could not read a file. That is
// unknown treated as none, which is the failure this project keeps
// finding. Raised in review on #215.
func PendingDecisions(root string) (qs []Question, notes []string, err error) {
	qs, notes = decisionQuestions(root)
	if len(qs) == 0 {
		return nil, notes, nil
	}
	store, loadErr := answers.Load(root)
	if loadErr != nil {
		return nil, notes, loadErr
	}
	return Unanswered(qs, store), notes, nil
}
