# A `.ts` file in a repository with no eslint config is linted against procoder's baseline and reports a real finding.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

The most common TypeScript setup there is — a tsconfig and no eslint config — got no linting and a green gate. Done means it is linted against procoder's baseline.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A `.ts` file in a repository with no eslint config is linted against procoder's baseline and reports a real finding.

## Evidence

Three tests in internal/lint/tsbaseline_test.go: the generated config imports the entry by absolute path; a stub package proves the wiring end to end with a no-debugger finding at its line (runs in CI); and the real package proves a type annotation is not a syntax error. Proved by mutation: dropping the parser config yields `Parsing error: Unexpected token :`, and making lintTSBaseline return notChecked unconditionally kills the success-path test while the refusal test still passes — which is exactly the hole review found.
