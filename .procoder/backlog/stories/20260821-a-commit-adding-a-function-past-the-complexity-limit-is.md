# A commit adding a function past the complexity limit is blocked.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

Complexity was a thing `procoder maintain` would tell you about if you
thought to ask. Nobody asks. The function that needs splitting is easiest
to split while it is being written, and hardest six months later when it
is the one everybody is afraid of.

Done means the commit that adds it says so. Reported by default rather
than blocking, because a threshold that blocks by surprise stops work on
exactly the files that need the refactor; blocking when the repository
sets `[maintain] policy = "block"`.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A commit adding a function past the complexity limit is blocked.

## Evidence

- `go test ./internal/maintain/ -run TestComplexityIsReportedAtTheGateAndBlocksOnlyOnRequest` — ComplexityChanged reports a function over the limit in a changed file, and blocks when `[maintain] policy = "block"`. (#134)
