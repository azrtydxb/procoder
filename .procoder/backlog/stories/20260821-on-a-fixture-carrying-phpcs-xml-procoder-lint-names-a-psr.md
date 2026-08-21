# On a fixture carrying `phpcs.xml`, `procoder lint` names a PSR-12 finding with its file and line.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A project that chose phpcs gets phpcs. Done means its PSR-12 findings arrive with file and line through the parser procoder already has.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture carrying `phpcs.xml`, `procoder lint` names a PSR-12 finding with its file and line.

## Evidence

`go test ./internal/lint/ -run TestPhpcsFindingsAreRead`: PASS over output recorded from PHP_CodeSniffer 4.0.4. Proved by mutation: narrowing findingLine to reject the column makes every phpcs finding vanish. Also run end to end — `procoder lint bug.php` with only phpcs.xml present reported three PSR-12 findings.
