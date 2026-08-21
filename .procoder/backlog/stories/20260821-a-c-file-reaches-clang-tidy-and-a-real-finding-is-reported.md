# A C++ file reaches clang-tidy and a real finding is reported with its file and line.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

C and C++ were formatted and linted by nothing at all: the gate looked at a .cpp file and reported clean with no analysis having happened.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A C++ file reaches clang-tidy and a real finding is reported with its file and line.

## Evidence

Run end to end: `procoder lint bug.cpp` reports `bug.cpp:4 warning: The right operand of '+' is a garbage value [clang-analyzer-core.UndefinedBinaryOperatorResult]`. Verified with PATH stripped to /usr/bin:/bin, which also proves the keg-directory resolution review asked for. clang-tidy's trailing `note:` lines are filtered — one uninitialised variable arrived as three findings before.
