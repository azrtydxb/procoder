# With an unanswered decision recorded, the gate reports it, and a run carrying one does not present as having nothing outstanding.

Status: done
Created: 2026-08-26
Epic: decisions-reach-the-user
Sprint: 022-a-decision-the-agent-cannot-make-reaches-the-user

## Description

Without this the queue is decoration. The agent records a decision,
nothing reports it, the run presents as having nothing outstanding, and
the decision is lost exactly as it was when it lived in prose.

Done when a waiting decision is visible where the verdict is read.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With an unanswered decision recorded, the gate reports it, and a run carrying one does not present as having nothing outstanding.

## Evidence

`TestTheGateReportsAnOutstandingDecision`, and its paired
`TestTheGateIsQuietWithNoDecision` so the first cannot pass by the gate
always reporting something. Killed by removing the `decisionQuestions`
call, and by removing the empty-pending early return respectively.

This fell out of `GateFindings` → `Pending` → `Collect` rather than
needing new code — verified by running it, not assumed from the call
graph. Live: `info  .procoder/ask/QA.md  1 question(s) waiting on a human`.

`GateFindings`' own comment said "four domains" and now says five.
