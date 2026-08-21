# A debt marker with no revisit condition is reported at the gate.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

A `debt:` marker records a corner cut on purpose. The rule is that it
names the condition to revisit — otherwise it is not recorded debt, it is
a shortcut with no way back, and the ledger fills with entries nobody can
act on.

The moment to ask for that condition is while the author still knows why
they cut the corner. Done means the gate says it for the files the commit
carries. Reported rather than blocking: a deliberate shortcut is a
judgement the author is entitled to make, but not to make silently.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A debt marker with no revisit condition is reported at the gate.

## Evidence

- `go test ./internal/debt/ -run TestAMarkerWithNoRevisitConditionIsReportedAtTheGate` — a marker with no revisit condition in a changed file is reported with its file and line, non-blocking; one that names its condition says nothing. Red when the changed-file filter is dropped. (#138)
