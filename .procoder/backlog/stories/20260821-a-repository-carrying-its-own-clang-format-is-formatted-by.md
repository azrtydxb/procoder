# A repository carrying its own `.clang-format` is formatted by that file, asserted by a style that differs from the baseline.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

D-OVERRIDE on the tool that used to demand a config: the fallback exists so an unconfigured repo is still checked, not so procoder's taste beats the project's.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A repository carrying its own `.clang-format` is formatted by that file, asserted by a style that differs from the baseline.

## Evidence

`go test ./internal/format/ -run TestARepositoryClangFormatWinsOverTheBaseline`: PASS. The fixture sets ColumnLimit: 20, which no builtin style would produce, so a wrapped result distinguishes "the project's file was read" from "a builtin was used". Proved by mutation: dropping --style=file leaves the line unwrapped.
