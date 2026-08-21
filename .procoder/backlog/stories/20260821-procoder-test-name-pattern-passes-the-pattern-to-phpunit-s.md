# `procoder test --name <pattern>` passes the pattern to phpunit's `--filter` and the reported count reflects the narrowed run.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A developer narrowing a run must get a narrowed run or be told they did not. Done means the pattern reaches --filter, and a pattern phpunit would misread is reported as NOT filtered rather than worn as a label over a whole-suite run.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder test --name <pattern>` passes the pattern to phpunit's `--filter` and the reported count reflects the narrowed run.

## Evidence

`go test ./internal/testrun/ -run TestADashPatternIsNotSilentlyPassedToPhpunit`: PASS — an ordinary pattern reaches `--filter`, a pattern beginning with `-` is not reported as filtered. Run end to end: `procoder test --name testPasses` reported `pass (1 test(s)) — filtered to "testPasses"` against a 2-test suite.
