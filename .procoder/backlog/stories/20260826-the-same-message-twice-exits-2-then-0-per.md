# The same message twice exits 2 then 0, per `TestTheSameMessageNeverBlocksTwice`; fails if the state write is removed.

Status: done
Created: 2026-08-26
Epic: an-unasked-decision-does-not-end-the-turn
Sprint: 025-an-unasked-decision-does-not-end-the-turn

## Description

The failure this exists for. A decision put to the user in the last paragraph of a report, with the work continuing underneath it — which is what the agent that wrote the rule did, hours after shipping it.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The same message twice exits 2 then 0, per `TestTheSameMessageNeverBlocksTwice`; fails if the state write is removed.

## Evidence

`hook.unaskedDecision` and `hook.decisionInProse` in `internal/hook/unasked.go`, wired into `Stop`. Proved by `TestAProseDecisionDoesNotEndTheTurn`, which also requires the reason to be actionable — it must name `decisions.md`, the structured question tool, and the fact that it will not fire twice. A block nobody can satisfy is the thing this is meant to prevent, not cause.

Verified against the real binary with a real payload shape: exit 2, reason on stderr.
