# A debt marker with no revisit condition, in a file the commit did not touch, is caught by CI and not by the gate — asserted so the two tiers cannot silently collapse into one.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

The two tiers answer different questions — the gate about the change, CI
about the tree — and the failure mode is that they quietly become one.
Either the gate grows into a whole-tree scan that reports the same eight
markers on every commit until people learn to skip the output, or CI
narrows to the change and stops catching what nobody has touched in a
year.

Done means the split is asserted rather than assumed: a marker in a file
the commit did not touch produces nothing at the gate, and is caught by
the whole-tree pass.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A debt marker with no revisit condition, in a file the commit did not touch, is caught by CI and not by the gate — asserted so the two tiers cannot silently collapse into one.

## Evidence

- `go test ./internal/debt/ -run TestAnUntouchedMarkerIsCIsNotTheGates` — a marker in an untouched file produces no gate finding, while Run over the whole tree names it AND exits non-zero so the CI step fails. Red on both halves: matching NoTrigger without the changed set, and restoring the unconditional `return 0`. (#138)
