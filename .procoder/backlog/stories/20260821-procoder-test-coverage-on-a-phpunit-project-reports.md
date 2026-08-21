# `procoder test --coverage` on a phpunit project reports coverage NOT measured rather than a number.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

Coverage needs xdebug or pcov in the PHP runtime, which procoder does not install and cannot assume. Done means the answer is "not measured", never a number nobody measured.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder test --coverage` on a phpunit project reports coverage NOT measured rather than a number.

## Evidence

`go test ./internal/testrun/ -run TestCoverageIsReportedNotMeasured`: PASS — Coverage stays below zero and no percentage appears in the detail. Run end to end: `procoder test --coverage` reported `pass (2 test(s)) — coverage not measured (phpunit needs xdebug or pcov in the PHP runtime)`.
