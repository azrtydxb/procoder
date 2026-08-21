# With `php` absent from a stub PATH, `procoder lint` over a `.php` file prints a NOT-checked line naming php and never reports clean.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A machine without php must be told the file was NOT checked. Done means the absence is loud, because silence is indistinguishable from clean.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With `php` absent from a stub PATH, `procoder lint` over a `.php` file prints a NOT-checked line naming php and never reports clean.

## Evidence

`go test ./internal/lint/ -run TestWithoutPHPTheFloorSaysNotChecked`: PASS — PATH set to an empty directory so a locally installed php cannot answer, and the single finding says NOT checked and names php. Proved by mutation: returning nil instead of notChecked makes every PHP file on such a machine report clean.
