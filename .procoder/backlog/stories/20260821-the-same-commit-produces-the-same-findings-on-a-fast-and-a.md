# The same commit produces the same findings on a fast and a slow run, asserted by running the gate twice against stubs of different speeds and comparing the output.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

The companion assertion to the no-budget rule, stated as the property a
budget would take away: run the same commit twice, changing only how long
the scanner takes, and compare.

Done means the two outputs are identical, asserted by comparison rather
than by inspection — a test that merely checks "some finding appeared" in
both runs would pass against a budget that dropped half of them.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The same commit produces the same findings on a fast and a slow run, asserted by running the gate twice against stubs of different speeds and comparing the output.

## Evidence

- `go test ./internal/security/ -run TestFastAndSlowRunsAgree` — two runs differing only in a 2s stub delay produce identical findings. Red against a 1s ceiling: the two runs disagree. (#137)
