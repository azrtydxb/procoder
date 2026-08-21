# Rule files that have drifted from AGENTS.md block the gate, making the sentence already in docs/commands.md true.

Status: done
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Rule files that have drifted from AGENTS.md block the gate, making the sentence already in docs/commands.md true.

## Evidence

- `go test ./internal/portability/` — AgentsDrift returns blocking findings for a drifted host file, nil when no AGENTS.md exists, and blocks on an unreadable one. (#130)
