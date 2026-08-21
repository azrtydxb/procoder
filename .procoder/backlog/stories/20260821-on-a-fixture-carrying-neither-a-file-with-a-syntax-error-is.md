# On a fixture carrying neither, a file with a syntax error is reported with its line, and a file that parses produces no finding at all.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A project that configured no linter still gets its syntax errors caught, and no opinion about style it never asked for. Done means a broken file is reported with its line and a file that parses says nothing at all.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture carrying neither, a file with a syntax error is reported with its line, and a file that parses produces no finding at all.

## Evidence

`go test ./internal/lint/ -run TestPHPSyntaxErrorsAreReportedAndCleanFilesAreSilent`: PASS over output recorded from php 8.5.6 — one finding for the parse error despite php echoing it twice, and NOT checked for output the parser does not recognise. Also run end to end: `procoder lint broken.php` reported broken.php:2, `procoder lint messy.php` reported 0 findings.
