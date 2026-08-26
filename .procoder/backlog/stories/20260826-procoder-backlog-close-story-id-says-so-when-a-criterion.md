# `procoder backlog close story <id>` says so when a criterion names no observable, asserted by `TestAStoryCriterionWithNoObservableIsCalledOut`. It reports rather than refuses: the refusal belongs at the draft spec, before the sprint opens, and refusing at close would retrofit the rule onto every story already in flight.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

The close controller is what asks for evidence against a story's criteria, so it is where a criterion nobody can run shows up as a practical problem.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder backlog close story <id>` says so when a criterion names no observable, asserted by `TestAStoryCriterionWithNoObservableIsCalledOut`. It reports rather than refuses: the refusal belongs at the draft spec, before the sprint opens, and refusing at close would retrofit the rule onto every story already in flight.

## Evidence

`TestAStoryCriterionWithNoObservableIsCalledOut`. It REPORTS rather than refuses, and that is the design: the refusal belongs at the draft spec, before the sprint opens. Refusing at close would apply a rule written today to every story already in flight — the retrofit this spec put out of scope. Killed by making `UncheckableCriteria` return nil.
