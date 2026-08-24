# `procoder review` over a fixture diff prints all five lenses with the content in scope, and leaves every file's bytes unchanged, asserted by comparing a digest of the tree before and after.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 012-review-with-judgment-not-just-tooling

## Description

The gate reads formatting, secrets, linters and hygiene — every one of
them mechanical. Nothing in procoder applies judgment to a change, so
whether anyone asks "is this the right shape, what breaks at the edges"
depends on who happens to be looking.

Done means `procoder review` exists and prints all five lenses over the
content in scope, for the agent to judge. And it writes nothing: the
binary prints, the agent decides — the same contract `procoder format`
already holds, asserted by comparing a digest of the tree before and
after.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder review` over a fixture diff prints all five lenses with the content in scope, and leaves every file's bytes unchanged, asserted by comparing a digest of the tree before and after.

## Evidence

- `go test ./internal/review/ -run TestReviewPrintsEveryLensAndWritesNothing` — all five lens bodies reach the output with the scope named, and a SHA-256 digest of every file's path and content is identical before and after. Verified end to end against a real git fixture: `procoder review` printed five lenses over the changed file, exit 0, tree unchanged. Mutation proven: Print writing a file alongside its output fails the digest while the printed bytes stay identical.
