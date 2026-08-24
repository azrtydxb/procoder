# `[planning] method = "nonsense"` is a config Problem naming the line, and the run continues on the default.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

Every other key in config.toml names the line when its value means
nothing, because a writer who mistypes a setting believes it is set. This
one is no different, and it is more consequential than most: a typo here
silently decides which methodology governs the repository.

Done means an unrecognised value is a Problem naming the line, and the
run continues on the default.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `[planning] method = "nonsense"` is a config Problem naming the line, and the run continues on the default.

## Evidence

- `go test ./internal/config/ -run TestAnUnknownPlanningMethodIsAProblemAndTheDefaultRuns` — exactly one Problem naming line 2 and listing `procoder, bmad`; the run continues on the default; both documented values are accepted and neither is a Problem. Verified end to end: `procoder config` printed `NOT applied .procoder/config.toml:2`, and the exit code matches an existing bad value (`security.sast_blocks_at = "NONSENSE"`) rather than being special-cased.
