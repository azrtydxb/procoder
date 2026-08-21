# `procoder test` on a fixture with `phpunit.xml` and a passing suite reports passed with its test count; the same fixture with one failing test reports failed and exits 1.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A PHP team's suite must run. Done means phpunit is detected from its config, the counts are read, and a failing suite fails the command.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder test` on a fixture with `phpunit.xml` and a passing suite reports passed with its test count; the same fixture with one failing test reports failed and exits 1.

## Evidence

`go test ./internal/testrun/ -run 'TestAPassingRunReportsItsCount|TestAFailedRunCountsErrorsAsWellAsFailures|TestPhpunitIsDetectedByEitherConfigSpelling'`: PASS over output recorded from phpunit 13.3. Proved by mutation: dropping the Errors term from phpunitCounts reports 2 passed/1 failed for a suite with one failure and one error. Run end to end: a passing fixture reported `ok php pass (2 test(s))`; adding a failing and an erroring test reported `FAIL php FAILED (1 passed, 2 failed)` with exit 1.
