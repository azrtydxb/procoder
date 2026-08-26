# The universal checks still block in anybody's repository

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

The point of narrowing is not to stop checking. A credential, a 12MB
binary, a `<<<<<<<` marker, a `.DS_Store`, a trailer nobody wrote — none of
those are matters of taste, and no repository wants them.

These stay blocking wherever procoder runs. If narrowing the gate turned
them into advice it would have traded a noisy gate for a useless one.

## Acceptance criteria

- [ ] In a non-adopting repository, a planted secret, an oversized file, a
      conflict marker, a junk file and an AI-attribution line each still
      block.

## Evidence
