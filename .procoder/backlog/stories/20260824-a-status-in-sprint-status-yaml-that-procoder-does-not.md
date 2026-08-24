# A status in `sprint-status.yaml` that procoder does not recognise is reported by name rather than mapped to a procoder status.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

BMad owns its status vocabulary and may extend it. Mapping an unknown
status to the nearest procoder equivalent — deciding `blocked` is close
enough to `open` — is how a status machine quietly loses a state, and the
report then misrepresents work nobody can see is misrepresented.

Done means an unrecognised status is reported by name.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A status in `sprint-status.yaml` that procoder does not recognise is reported by name rather than mapped to a procoder status.

## Evidence

- `go test ./internal/planning/ -run TestAnUnknownStatusIsReportedByName` — `blocked` is reported quoted by name, non-blocking, and the known status beside it is silent. Verified end to end. Mutation proven: dropping the Known check and mapping unrecognised statuses to `backlog` reports a blocked story as not yet started, and the finding that would have said so never appears.
