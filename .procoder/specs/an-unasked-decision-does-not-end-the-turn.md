# An unasked decision does not end the turn

Status: complete

## Problem

3.2.0 shipped the rule that a decision is not the agent's to make: stop,
record it in `.procoder/ask/decisions.md`, put it to the user with the
host's structured question tool, do not bury it at the end of a report.

Hours later, in this repository, the agent that wrote the rule ended a
triage report with three decisions in prose and continued. The rule was in
the tool and not in the agent. `procoder ask` can make a recorded decision
visible; nothing makes an agent record one.

That is the shape of every other failure this project has fixed. A rule
that depends on remembering is a rule that fails on the day somebody is
busy — which is the argument the credit check, the falsifier rule and the
release controller each already won.

The moment it fails is identifiable: the end of the turn. An unasked
decision is one the agent is about to hand back inside prose, having done
the work around it.

## Users

The person, who is currently asked for decisions in the last paragraph of
a long report and has to notice, scroll back, and reconstruct what is
being decided.

The agent, which has a rule it cannot check itself.

## In scope

- [S-1] The Stop hook reads `last_assistant_message` from the host payload
  and detects a decision put to the user in prose — `internal/hook/stop.go`.
- [S-2] When one is found and `.procoder/ask/decisions.md` holds no
  unanswered decision, the turn does not end: the hook exits 2 with a
  reason on stderr, which is the documented way a Stop hook continues the
  conversation.
- [S-3] When a decision IS recorded, the hook is silent. The agent did the
  thing; being told about it anyway is how a check stops being read.
- [S-4] The detector is conservative. It fires on an explicit ask —
  "should I", "do you want", "let me know", "your call" — not on a
  question mark, because a report containing a rhetorical question is not
  a decision put to anybody.
- [S-5] The same message never blocks twice. A block the agent cannot
  satisfy would end the session in a loop, which is worse than the failure
  it is preventing.
- [S-6] The handoff note this hook already writes is unaffected on every
  path, including the blocking one.

## Out of scope

- Reading the transcript. The host documents it as written asynchronously
  and lagging the current turn, and names `last_assistant_message` as the
  field for exactly this. Parsing it anyway would be building on the thing
  the documentation warns about.
- Making the agent ask WELL. The hook can see that a decision was buried;
  it cannot judge whether the options offered are the right ones.
- Any other host. The Stop event and its payload are Claude Code's; a host
  without them gets the rule as prose, as today.

## Constraints

- **A false block teaches bypass.** This is the failure behind #172 and
  #185, and a Stop hook that fires on ordinary reports would be the worst
  instance of it yet. The detector errs toward silence.
- **A session must never fail to end.** Every path exits 0 unless it is
  deliberately blocking, and no message blocks twice.
- **The handoff note is not sacrificed.** It is written before any
  blocking decision is taken.

## Interfaces

| Surface                                                 | Behaviour                                                    |
| ------------------------------------------------------- | ------------------------------------------------------------ |
| Stop payload with a prose decision, nothing recorded    | exit 2, reason on stderr, the turn continues                 |
| Stop payload with a prose decision, a decision recorded | silent, exit 0                                               |
| Stop payload with no decision-shaped ask                | silent, exit 0                                               |
| The same message a second time                          | silent, exit 0                                               |
| No `last_assistant_message` in the payload              | silent, exit 0 — an absent field is not evidence of anything |

## Data

One line of state under `.procoder/state/`: a hash of the message this
hook last blocked on, so the same message cannot block twice.

## Edge cases

- **The payload has no `last_assistant_message`.** Older host, another
  host, or a malformed event. Silence: an absent field is not evidence a
  decision was buried.
- **A decision is recorded but already answered.** Not outstanding, so it
  does not excuse a new one buried in prose. The check asks for an
  UNANSWERED decision.
- **The agent asks properly AND writes a summary containing "let me
  know".** A recorded decision makes the hook silent, so asking properly
  is always the way out.
- **`.procoder/ask/decisions.md` is unreadable.** Silence rather than a
  block: procoder cannot tell whether a decision was recorded, and
  blocking on not-knowing would be a block nobody can act on.
- **A repository that never adopted procoder.** The hook writes no note
  and blocks nothing there, as today.

## Failure modes

- **It fires on an ordinary report.** The one that matters. Held by an
  explicit-ask detector rather than a question mark, and by a test over
  real reports that must stay silent.
- **It blocks in a loop.** Held by the one-block-per-message state, and by
  a test that runs the same message twice.
- **It swallows the handoff note.** Held by writing the note first and
  asserting it on the blocking path.

## Acceptance criteria

- [ ] [S-1] [S-2] A Stop payload whose `last_assistant_message` ends with
      "Do you want me to X or Y?" and a repository with no unanswered
      recorded decision exits 2 and names the fix on stderr, per
      `TestAProseDecisionDoesNotEndTheTurn`; fails if the detector is
      made to return false.
- [ ] [S-3] The same payload, with an unanswered decision recorded, exits
      0 and says nothing, per `TestARecordedDecisionIsSilent`; fails if
      the recorded-decision check is dropped.
- [ ] [S-4] Ordinary reports — a status summary, a report containing a
      rhetorical question, a completed-work message — all exit 0, per
      `TestOrdinaryReportsDoNotBlock`; fails if the detector fires on a
      question mark.
- [ ] [S-5] The same message twice exits 2 then 0, per
      `TestTheSameMessageNeverBlocksTwice`; fails if the state write is
      removed.
- [ ] [S-6] The handoff note is written on the blocking path too, per
      `TestTheHandoffNoteSurvivesABlock`; fails if the block returns
      before the note is written.
- [ ] [S-1] A payload with no `last_assistant_message` exits 0, per
      `TestNoMessageIsNotEvidence`; fails if an absent field is treated as
      an empty ask.

## Open questions

## Decisions

- **`last_assistant_message`, not the transcript.** The host documents the
  transcript as lagging the current turn and names this field as the one
  for it. Checked against the documentation rather than assumed, after the
  first design read the transcript.
- **Exit 2 with stderr, not a JSON decision.** That is what the host
  documents as the Stop hook's blocking mechanism.
- **Conservative detection.** A missed burial costs one prose question. A
  false block costs the credibility of every check procoder makes.
