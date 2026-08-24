# No procoder-owned feature name contains "BMad", asserted by an audit over the source and the command table, so the trademark boundary cannot erode by accident.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 012-review-with-judgment-not-just-tooling

## Description

"BMad" and "BMAD-METHOD" are trademarks of BMad Code, LLC. Naming BMad to
describe interoperation — a setting value, a doctor line, a sentence in
the docs — is fine. Naming a procoder feature after it is not, and that
boundary erodes by accident rather than by decision: someone adds
`procoder bmad-sync` because it is the clearest name available.

Done means an audit over the source and the command table catches it, so
the boundary is held by a test rather than by everyone remembering.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] No procoder-owned feature name contains "BMad", asserted by an audit over the source and the command table, so the trademark boundary cannot erode by accident.

## Evidence

- `go test ./internal/audit/ -run TestNoProcoderFeatureIsNamedAfterATrademark`
  — audits the command table in `cmd/procoder/main.go` and every exported
  `func`/`type` under `internal/`. Four mutations proven failing: a
  `case "bmad-sync":` arm (caught at main.go:446), an exported func, an
  exported type, and a method on a receiver (each caught by file and line).
  A fifth probe proved the distinction holds in the other direction — a
  config value `"bmad"`, a doctor string naming BMad, and an unexported
  helper are all permitted, because naming the mark to describe
  interoperation is nominative use and only a feature NAME is a claim.
