# On a fixture carrying `phpstan.neon`, `procoder lint` over a file with a wrong return type names the file, the line, and the message, parsed from phpstan's raw format including its missing space.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A project that chose phpstan gets phpstan, run against its own config and level. Done means findings arrive with file, line and message — including through phpstan's raw format, which the shared parser cannot read.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture carrying `phpstan.neon`, `procoder lint` over a file with a wrong return type names the file, the line, and the message, parsed from phpstan's raw format including its missing space.

## Evidence

`go test ./internal/lint/ -run TestPhpstanFindingsSurviveTheMissingSpace`: PASS over output recorded from phpstan 2.2.8. Proved by mutation: requiring the space in phpstanLine (`:(\d+):\s+`) drops both findings and the file reports clean. Also run end to end — `procoder lint bug.php` in a project with phpstan.neon named bug.php:3 and bug.php:5 with the [identifier=...] suffix stripped.
