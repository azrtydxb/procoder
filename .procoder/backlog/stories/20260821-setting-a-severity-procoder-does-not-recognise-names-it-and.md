# Setting a severity Procoder does not recognise names it and uses the default, and the run still reports findings.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

A typo in a severity must not quietly disable blocking. Done means it is named, the default stays in force, and findings still report.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Setting a severity Procoder does not recognise names it and uses the default, and the run still reports findings.

## Evidence

`go test ./internal/config/ -run TestAnUnrecognisedSeverityIsNamedAndTheDefaultUsed`: PASS — `sast_blocks_at = "SEVERE"` leaves ERROR in force and produces one Problem naming severity. Run end to end: `procoder config` printed `NOT applied ... not a severity semgrep reports (INFO, WARNING, ERROR)`.
