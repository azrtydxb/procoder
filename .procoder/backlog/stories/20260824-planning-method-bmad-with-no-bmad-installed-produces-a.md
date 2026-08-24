# `[planning] method = "bmad"` with no BMad installed produces a blocking finding naming both the setting and the missing installation.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

Silently falling back to procoder's own chain would leave a repository
believing BMad governs its planning while procoder quietly governed it
instead — and the first they would learn of it is a report that does not
match the artifacts on disk.

Done means the mismatch is named: a blocking finding citing both the
setting and the missing installation.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `[planning] method = "bmad"` with no BMad installed produces a blocking finding naming both the setting and the missing installation.

## Evidence

- `go test ./internal/planning/ -run TestAChosenMethodThatIsNotInstalledBlocks` — a blocking finding naming both the setting and the missing installation, and the default method produces no planning findings at all. Verified end to end: `procoder check` printed the block on a fixture with `method = "bmad"` and no `_bmad/`. Mutation proven: returning nil when the install is absent greens the gate and governs the repository by a methodology it did not choose.
