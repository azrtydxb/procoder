# A language procoder formats but cannot lint reports NOT checked, blocking, naming the language — never nothing.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

C# and Dart are formatted and have no linter. Done means the gate says so rather than counting the file as clean.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A language procoder formats but cannot lint reports NOT checked, blocking, naming the language — never nothing.

## Evidence

`go test ./internal/lint/ -run TestEveryFormattedExtensionReachesALinterOrSaysItDoesNot` covers .cs and .dart. Run end to end — `procoder lint A.cs` prints `BLOCK A.cs NOT linted — C#: procoder has no linter for it yet` and the run reports 1 blocking.
