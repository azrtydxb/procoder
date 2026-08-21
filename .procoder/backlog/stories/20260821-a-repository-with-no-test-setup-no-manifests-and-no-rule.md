# A repository with no test setup, no manifests and no rule files commits without any new blocking finding.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

This sprint gives the gate five new legs. Each has to decide what to say
about a repository that has nothing for it to look at, and the wrong
answer is easy to reach one leg at a time: each is individually defensible
and together they make an empty repository unable to commit a text file.

Done means a repository with no test setup, no manifests and no rule
files commits with no new blocking finding — "nothing here" comes out
silent. A machine missing the scanners still blocks, loudly and on
purpose; that is a different question and has its own test.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A repository with no test setup, no manifests and no rule files commits without any new blocking finding.

## Evidence

- `go test ./internal/gate/ -run TestAQuietRepositoryStillCommits` — a temp repo holding one NOTES.txt exits 0 with no BLOCKING line. Red when AgentsDrift blocks on a missing AGENTS.md. (#137)
