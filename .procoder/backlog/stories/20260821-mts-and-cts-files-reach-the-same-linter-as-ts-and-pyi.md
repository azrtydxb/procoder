# `.mts` and `.cts` files reach the same linter as `.ts`, and `.pyi` reaches ruff, asserted by a fixture of each.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

`.mjs` and `.cjs` linted while `.mts` and `.cts` did not, and `.pyi` was formatted and never linted. Silence by omission, not by decision.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `.mts` and `.cts` files reach the same linter as `.ts`, and `.pyi` reaches ruff, asserted by a fixture of each.

## Evidence

`go test ./internal/lint/ -run TestEveryFormattedExtensionReachesALinterOrSaysItDoesNot`: PASS over .mts, .cts, .pyi, .cpp, .c, .cs and .dart. Proved by mutation: removing .mts/.cts from the dispatch reports `a.mts reaches no linter and reports nothing — a silent pass`. Run end to end — `procoder lint bad.mts bad.cts` returned four findings where it previously returned none.
