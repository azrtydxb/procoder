# A Stop payload whose `last_assistant_message` ends with "Do you want me to X or Y?" and a repository with no unanswered recorded decision exits 2 and names the fix on stderr, per `TestAProseDecisionDoesNotEndTheTurn`; fails if the detector is made to return false.

Status: done
Created: 2026-08-26
Epic: an-unasked-decision-does-not-end-the-turn
Sprint: 025-an-unasked-decision-does-not-end-the-turn

## Description

The failure this exists for. A decision put to the user in the last paragraph of a report, with the work continuing underneath it — which is what the agent that wrote the rule did, hours after shipping it.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A Stop payload whose `last_assistant_message` ends with "Do you want me to X or Y?" and a repository with no unanswered recorded decision exits 2 and names the fix on stderr, per `TestAProseDecisionDoesNotEndTheTurn`; fails if the detector is made to return false.

## Evidence

`hook.unaskedDecision` and `hook.decisionInProse` in `internal/hook/unasked.go`, wired into `Stop`. Proved by `TestAProseDecisionDoesNotEndTheTurn`, which also requires the reason to be actionable — it must name `decisions.md`, the structured question tool, and the fact that it will not fire twice. A block nobody can satisfy is the thing this is meant to prevent, not cause.

Verified against the real binary with a real payload shape: exit 2, reason on stderr.
