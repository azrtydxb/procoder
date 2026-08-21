# On a fixture carrying both configs, findings from both tools appear.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A project that configured both linters gets both. Done means neither tool's findings are lost because the other ran.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture carrying both configs, findings from both tools appear.

## Evidence

Run end to end with phpstan.neon and phpcs.xml both present: `procoder lint bug.php` reported 5 findings — 2 from phpstan (lines 3 and 5) and 3 from phpcs (lines 1, 1, 2). The selection logic is covered by `go test ./internal/lint/ -run TestTheConfiguredLinterIsFound`, which walks up from the file for each config spelling.
