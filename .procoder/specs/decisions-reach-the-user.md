# A decision the agent cannot make reaches the user, and survives

Status: complete

## Problem

Procoder's principles say questions are not the agent's to answer, and
`procoder ask` collects them. Both are true only for questions that come
from a **finding** — an undecided spec question, a lint result that may be
a false positive, a flag on something that may be a test credential.
`ask.Collect` gathers exactly four such sources, all computed from
repository state.

A **decision** — commit or hold, merge now or after, file this or let it
go, which of two approaches — has no path anywhere. It does not come from
a finding; it comes from the work. Nothing collects it, nothing records it,
and nothing notices it went unasked.

What happens instead was demonstrated in this repository on 2026-08-26.
Two decisions were put to the user as prose at the end of a long status
report, and the work continued underneath them across four more messages.
The user's response was that they had not been asked properly. They were
right, and the reason is that there was no way to do it properly: no rule
named the case, and `AskUserQuestion` — the host tool that renders a
selectable question — appears in no procoder rule file at all.

The failure is quiet in the way this project cares about. An unanswered
decision does not block anything, does not appear in any report, and does
not survive compaction. It is simply lost, and the agent proceeds on its
own preference — which is the same failure as an invented answer to a
finding, arriving by a route nobody guarded.

## Users

The AI coder, which needs somewhere to put a decision it must not make.

The person, who needs decisions to arrive one at a time and legibly rather
than buried at the end of a report they have to reconstruct.

The gate, which needs to know a decision is outstanding so that "everything
is green" cannot be said while a question waits.

## In scope

- [S-1] The agent can record a decision it cannot make, as a file it
  writes, and `procoder ask` collects it alongside the four finding-derived
  sources.
- [S-2] The binary never writes that file. P-CONTROL is unchanged: the
  agent writes, the binary reads and prints.
- [S-3] An outstanding decision is visible to the gate the way pending
  questions already are, so a run cannot report clean while one waits.
- [S-4] An answered decision stays answered across sessions, keyed the way
  finding-derived answers already are, so the next session starts from the
  decision rather than asking again.
- [S-5] The principles state that a decision is not the agent's to make,
  that STOP means asking before continuing rather than mentioning at the
  end, and that the host's structured question tool is used where one
  exists.

## Out of scope

- Making the agent's compliance mechanical. Nothing here can force an agent
  to ask; the queue makes an unasked decision _visible_ and an asked one
  _durable_. That is the whole claim, and overstating it would be the same
  silent green in a different place.
- A procoder-side question UI. The host already has one; procoder's job is
  the record, not the prompt.
- Retrofitting the four existing sources. They work.

## Constraints

- **P-CONTROL.** The binary prints, the agent writes. A decision queue that
  the binary writes to would break the rule the whole tool is built on.
- **No silent green.** A waiting decision must be reported, or the queue is
  decoration.
- **Nothing new is invented about answers.** Decisions are keyed, stored
  and answered by the machinery `answers` already provides.

## Interfaces

| Surface                      | Behaviour                                                                                               |
| ---------------------------- | ------------------------------------------------------------------------------------------------------- |
| `.procoder/ask/decisions.md` | Written by the agent. Each entry is one decision with its options. Read by `Collect` as a fifth source. |
| `procoder ask`               | Lists it with the other four, indistinguishable in handling.                                            |
| `procoder ask --file <path>` | Records the answer, unchanged.                                                                          |
| The gate                     | Reports an outstanding decision as it already reports pending questions.                                |

## Data

One new file, written by the agent, in the directory the ask domain already
owns. No new store, no new key scheme — `answers.Key(source, origin, text)`
with a `decision` source.

## Edge cases

- **The file does not exist.** The overwhelmingly common case, and not a
  finding. Contributes nothing, exactly as a domain with nothing to ask.
- **The file exists and is malformed.** A note, not silence — the same rule
  the other collectors follow, because "unreadable" and "empty" must not
  look the same.
- **A decision is answered and left in the file.** Filtered by
  `Unanswered`, like every other answered question.
- **A decision's text is edited after being answered.** The key changes, so
  it asks again. Correct: an edited question is a different question.

## Failure modes

- **The queue becomes a way to avoid asking.** An agent writes the decision
  down, reports "recorded", and proceeds anyway. The principle addresses
  this and the queue cannot; S-3's gate reporting is what makes the
  avoidance visible rather than silent.
- **The binary gains a write path by accident**, through a helpful
  `--raise` flag. Held by a test asserting the binary writes nothing.

## Acceptance criteria

- [ ] [S-1] A decisions file the agent wrote appears in `procoder ask`
      output alongside spec, docs, security and lint questions.
- [ ] [S-2] Running every `ask` command against a repository with a
      decisions file leaves every file on disk byte-identical, the
      decisions file included.
- [ ] [S-3] With an unanswered decision recorded, the gate reports it, and
      a run carrying one does not present as having nothing outstanding.
- [ ] [S-4] An answered decision does not reappear on the next run, and
      the same decision with edited text does.
- [ ] [S-5] `procoder principles` states the decision rule, and a test
      pins that it does — the rule is the deliverable, so its absence must
      fail rather than be noticed later.
- [ ] [S-1] A missing decisions file produces no finding and no note; a
      malformed one produces a note naming the file.

## Open questions

## Decisions

- **The agent writes the file, not the binary.** The alternative — a
  `procoder ask --raise "..."` that writes — reads better on the command
  line and breaks P-CONTROL. Rejected for that reason alone.
- **Decisions live with questions rather than in their own command.** They
  are answered by the same person through the same file and stored by the
  same keys; a parallel command would duplicate all of it to express a
  distinction the user does not experience.
