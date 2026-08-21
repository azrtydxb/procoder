# `procoder format` on a fixture `.php` file prints the formatted source, exits 0, and leaves the file's bytes unchanged — asserted by comparing the file's digest before and after.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A PHP developer writes a `.php` file and expects the same contract every other language gets: procoder hands back the formatted source and never edits the file behind them. Done means .php reaches a formatter at all, and that formatter prints.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder format` on a fixture `.php` file prints the formatted source, exits 0, and leaves the file's bytes unchanged — asserted by comparing the file's digest before and after.

## Evidence

`go test ./internal/format/ -run TestFormattingPHPNeverTouchesTheFile` with the real @prettier/plugin-php installed: PASS, the file's bytes identical before and after. Proved by mutation: adding an os.WriteFile of the formatted result to format.Check makes it fail with "formatting must print the result, never write it". Also run by hand against a messy fixture — `procoder format messy.php` printed the corrected source and `md5 -q` was unchanged.
