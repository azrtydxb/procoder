# A repository with no test setup, no manifests and no rule files commits without any new blocking finding.

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

- [x] A repository with no test setup, no manifests and no rule files commits without any new blocking finding.

## Evidence

- `go test ./internal/gate/ -run TestAQuietRepositoryStillCommits` — a temp repo holding one NOTES.txt exits 0 with no BLOCKING line. Red when AgentsDrift blocks on a missing AGENTS.md. (#137)
