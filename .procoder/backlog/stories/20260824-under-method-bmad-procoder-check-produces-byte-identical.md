# Under `method = "bmad"`, `procoder check` produces byte-identical output to the same tree under `method = "procoder"`, asserted on a fixture — the setting governs planning and nothing else.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The seam this whole track rests on: planning moves, governance does not.
It is the only reason a BMad repository would install procoder at all, and
the tempting design — a spectrum where the setting dials procoder back by
degrees — erodes it one exception at a time.

Done means the gate's output is byte-identical across the setting on a
fixture, so the seam cannot drift without a test going red.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Under `method = "bmad"`, `procoder check` produces byte-identical output to the same tree under `method = "procoder"`, asserted on a fixture — the setting governs planning and nothing else.

## Evidence

- `go test ./internal/gitcmd/ -run TestGovernanceIsUntouchedByThePlanningMethod` — every finding the gate makes about the CODE is identical across both settings, the planning domain's own findings excluded because those are the setting working. Verified end to end: `procoder check a.go README.md` over one fixture produced byte-identical output under both methods (8 findings, `diff` clean), and the run was a full gate — format verdicts, lint, hygiene, docs, ask — not an early bail. Mutation proven: gating the docs obligation on cfg.Planning() breaks the comparison while every other test stays green.
