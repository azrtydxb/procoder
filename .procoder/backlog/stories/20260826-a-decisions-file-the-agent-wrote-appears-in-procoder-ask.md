# A decisions file the agent wrote appears in `procoder ask` output alongside spec, docs, security and lint questions.

Status: done
Created: 2026-08-26
Epic: decisions-reach-the-user
Sprint: 022-a-decision-the-agent-cannot-make-reaches-the-user

## Description

The fifth source. The other four are computed from repository state — a
spec's open questions, a docs obligation, a flagged secret, a lint
finding. A decision is not computed by anything: it comes from the work,
when the next step forks and the fork is not the agent's to pick.

Done when a decision the agent wrote down is indistinguishable, in
handling, from a question a domain raised.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A decisions file the agent wrote appears in `procoder ask` output alongside spec, docs, security and lint questions.

## Evidence

`internal/ask/decisions.go` — `decisionQuestions`, wired into `Collect`
as a fifth source. Proved by `TestRecordedDecisionsAreCollected`, which
asserts both the heading and the options survive: a decision presented
without its options is the prose question this change exists to replace.
Killed by removing the `decisionQuestions` call, and separately by
dropping the option lines.

Verified end to end against a scratch repository: two decisions written,
both collected, `QA.md` written with the options intact.
