# A C++ file in a repository with no `.clang-format` is formatted against procoder's baseline style and reported clean or unformatted, never out of scope.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

A C or C++ project with no style file was formatted by nothing and told it was fine, because out of scope counts as skipped and passes. Done means the file is formatted against a baseline named on the command line, with nothing written to the repo.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A C++ file in a repository with no `.clang-format` is formatted against procoder's baseline style and reported clean or unformatted, never out of scope.

## Evidence

`go test ./internal/format/ -run TestCWithoutProjectConfigIsStillFormatted`: PASS. Proved by mutation: restoring NeedsProjectConfig on clang-format returns OutOfScope and the test names the verdict. Run end to end — `procoder format a.cpp` in a directory with no .clang-format printed `int main() { return 0; }`.
