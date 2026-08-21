# An empty template file blocks rather than falling back to the default, asserted against the existing empty-documentation guard.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

Falling back quietly is the dangerous option, and not hypothetically: a strip-the-header pipeline empties an already-formatted file on the SUCCESS path, which destroyed docs/commands.md in this repository today.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] An empty template file blocks rather than falling back to the default, asserted against the existing empty-documentation guard.

## Evidence

`go test ./internal/templates/ -run TestAnEmptyTemplateBlocksInsteadOfFallingBackQuietly`: PASS for both empty and whitespace-only. Proved by mutation: treating whitespace-only as absent silently replaces the team's template. Run end to end — the emptied template printed the refusal with its `git checkout` line, Procoder still emitted a usable template for that run, and `procoder check` on the file exited 1 through the empty-documentation guard.
