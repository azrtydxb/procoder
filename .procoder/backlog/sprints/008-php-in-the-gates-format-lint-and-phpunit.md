# PHP in the gates: format, lint, and phpunit

Status: closed 2026-08-21
Created: 2026-08-21

## Goal

A PHP repository gets the same contract every other language gets from
procoder: the formatter prints the corrected file without touching it, the
linter runs whatever the project configured and falls back to a syntax
floor rather than an imposed style, phpunit runs as a detected suite, and
every tool that is absent says so instead of being counted as clean.

Today `procoder check` reports clean over a PHP tree it never looked at.
That is the failure this sprint closes: a green verdict that means only
that procoder does not speak the language.

## Stories

<!-- pulled below -->

## Result

committed: 12
done: 12 (20260821-a-repository-whose-node-modules-has-no-prettier-plugin-php, 20260821-on-a-fixture-carrying-both-configs-findings-from-both-tools, 20260821-on-a-fixture-carrying-neither-a-file-with-a-syntax-error-is, 20260821-on-a-fixture-carrying-phpcs-xml-procoder-lint-names-a-psr, 20260821-on-a-fixture-carrying-phpstan-neon-procoder-lint-over-a, 20260821-procoder-doctor-names-every-php-tool-it-looked-for-with-a, 20260821-procoder-format-on-a-fixture-php-file-prints-the-formatted, 20260821-procoder-test-coverage-on-a-phpunit-project-reports, 20260821-procoder-test-name-pattern-passes-the-pattern-to-phpunit-s, 20260821-procoder-test-on-a-fixture-with-phpunit-xml-and-a-passing, 20260821-the-documentation-s-language-table-lists-php-with-its-tools, 20260821-with-php-absent-from-a-stub-path-procoder-lint-over-a-php)
carried: 0

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
