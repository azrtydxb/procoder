# select OpenTofu for infrastructure checks

Status: closed 2026-09-15
Created: 2026-09-15

## Description

Fix #286: infra validation must use OpenTofu for its provider installation,
rather than falsely blocking unrelated commits with Terraform provider errors.
Share per-directory selection with doctor/init, retain blocking validation
failures, and report missing tools or unreadable evidence explicitly.

## Acceptance criteria

- [x] Registry evidence selects OpenTofu without falling back when tofu is missing.
- [x] Both fmt and validate use the selected binary and name it in findings.
- [x] doctor/init use the same per-directory selection.
- [x] Regression mutations fail, the full suite passes, and the gate is clean.

## Evidence

- `go test ./internal/infra` and the doctor regression tests pass; fixture tools verify fmt/validate arguments, missing-tool refusal, severity, and per-directory inventory.
- Five applied mutations failed their regression tests: unconditional Terraform selection, ignored lock read error, wrong execution binary, hardcoded doctor inventory, and omitted selection-error reporting. Each mutation was restored.
- `procoder test`: Go suite passes across 57 packages. JavaScript is NOT run because package.json has no test script.
- `procoder check`: zero blocking findings, zero unchecked files; static checks use the pinned tool versions. Existing complexity advisories remain.
- `procoder review --lens adversarial,edge-case` led to tighter error assertions and explicit doctor reporting of unreadable evidence.
- OpenCode and Kilo command parity tests pass after regeneration. No actual provider initialization or remote infrastructure access occurs in the regression tests.
