# interactive-qa: procoder asks the human instead of letting the coder guess

Status: closed 2026-08-21
Created: 2026-08-20

## Goal

Every domain here already knows what it cannot decide — a spec's open
question, a documentation gap nobody cleared, a flagged secret, a lint
finding that needs judgement. Today those reach the AI coder as prose, the
coder answers them itself, and the answer reads like a resolution rather
than the guess it is. The human never sees the question.

By the end of this sprint `procoder ask` collects those questions, puts
them to a person — on a terminal, or through a file when there is no
terminal to ask — and records the answers where both the coder and the
gate can read them. An answer outlives the session, a changed question is
asked again, and a flagged secret's value never appears in any of it.

Done means a question reaches a person instead of being invented, and the
decision survives the session that made it.

## Result

committed: 9
done: 9 (20260820-c-01-procoder-ask-on-a-fixture-carrying-one-question-per, 20260820-c-02-with-a-terminal-it-asks-one-question-at-a-time-with, 20260820-c-03-procoderaskqamd-and-procoderaskanswersmd-are-written, 20260820-c-04-procoder-ask---file-path-records-the-answers-in-that, 20260820-c-05-an-answer-persists-a-question-already-answered-is-not, 20260820-c-06-the-posttooluse-hooks-additionalcontext-carries-the, 20260820-c-07-procoder-ask-exits-1-while-any-question-is-unanswered, 20260820-c-08-ask-policy--block-makes-pending-questions-block, 20260820-c-09-a-spec-question-that-has-been-answered-in-answersmd-no)
carried: 0

## Retro

What slowed us down: the design contradicted itself in three places, and only
one of them was listed as an open question. The spec said a re-run clears
previous answers while the decision said answers persist; the plan's risk
table said the same thing in different words, which is the line an implementer
would have followed; and the plan's spec collector was written to parse
`OPEN:` lines only, when `spec check` blocks on anything left in that section
— so the human would have been unable to answer the very questions the gate
was refusing on. Reading the plan against the decisions before writing code
cost twenty minutes and saved all three.

What we change next sprint because of it: when a decision changes a spec,
grep the plan for the thing it changed before starting. A spec and its plan
are two documents and only one of them gets re-read.

One adaptation worth keeping: the sprint's own work found two bugs outside its
scope, and both were fixed at the root rather than worked around. `procoder
lint` had been reporting "NOT checked — golangci-lint failed: directory not
found" for the first changed file in every run, because `ChangedFiles` ran
`git status --porcelain` output through `TrimSpace` and ate the leading space
of the first line — `cmd/procoder/main.go` arrived as `md/procoder/main.go`.
The tool was blaming the tool for its own bad argument. Building a collector
on top of that list is what surfaced it: a feature that reads another
domain's output is a test of that domain.
