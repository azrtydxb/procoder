# `procoder spec check` over a fixture whose criterion reads "the gate is correct" refuses, naming that criterion and saying a criterion must name the command, test or artifact that observes it.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

A checker that refuses correct citations is one people switch off — the failure this project keeps relearning (#172, #185).

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder spec check` over a fixture whose criterion reads "the gate is correct" refuses, naming that criterion and saying a criterion must name the command, test or artifact that observes it.

## Evidence

`TestResolvingCitationsAreSilent` and `TestCitationsInsideFencesAreIgnored`. The first was written after a real false positive: `AGENTS.md` parsed as the `pkg.Symbol` shape, the symbol `md` was looked up, and a file sitting in the repository root was refused. Killed by emptying `fileExtensions` and by removing the fence strip respectively.
