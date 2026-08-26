# `procoder spec check` over a fixture spec citing `nosuchpkg.NoSuchSymbol` and `internal/nowhere/absent.go` refuses, exits 2, and names both citations with their line numbers.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

A claim that names something can be checked; one that names nothing cannot be checked by anybody, including its author.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder spec check` over a fixture spec citing `nosuchpkg.NoSuchSymbol` and `internal/nowhere/absent.go` refuses, exits 2, and names both citations with their line numbers.

## Evidence

`spec.UnresolvedCitations` resolves backticked `pkg.Symbol` and repository-path citations against the tree, parsing Go rather than grepping so a name that appears only in a comment does not count as existing. Proved by `TestUnresolvedCitationsAreReported`, killed by making `resolves` return true always.
