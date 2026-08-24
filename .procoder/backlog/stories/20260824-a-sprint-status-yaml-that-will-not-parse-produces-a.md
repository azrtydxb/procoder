# A `sprint-status.yaml` that will not parse produces a blocking finding naming the file, distinct from the finding for one that is absent.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

Unreadable and absent are different answers, and only one of them is the
repository's choice. A parse failure reported as "no sprint yet" sends
somebody to plan work that is already planned.

Done means the unparseable case blocks and names the file, distinct from
the finding for a repository that has not planned anything.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A `sprint-status.yaml` that will not parse produces a blocking finding naming the file, distinct from the finding for one that is absent.

## Evidence

- `go test ./internal/planning/ -run TestAStatusFileThatWillNotParseIsNotAnEmptySprint` — a file with no `development_status` block blocks and names the file, while an absent file is no finding at all and reports "no planning artifacts yet". Verified end to end on both fixtures. Mutation proven: returning nil findings for an unparseable file makes a repository mid-sprint read as one that has not started.
