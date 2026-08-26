# The same command over a spec citing `gitx.Attribution` and `internal/gate/gate.go` — both of which exist — is silent about citations.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

A criterion nobody can run is an agreement, not a test. It passes review, becomes a story, gets ticked, and the thing it promised was never checked by anything.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The same command over a spec citing `gitx.Attribution` and `internal/gate/gate.go` — both of which exist — is silent about citations.

## Evidence

`TestACriterionWithNoObservableIsReported`, killed by negating the `observableRe` test.

Also `TestAWrappedCriterionIsReadWhole`, which exists because reading only a bullet's first line was this checker's own first bug — it refused three criteria in its own spec whose observable sat on the continuation line.
