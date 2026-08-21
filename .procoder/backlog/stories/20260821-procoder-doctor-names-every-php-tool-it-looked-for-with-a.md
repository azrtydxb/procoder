# `procoder doctor` names every PHP tool it looked for, with a version when present and an install line when absent.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

Someone setting up a PHP repo needs to know what is missing and how to get it — and must not be sent to install a linter procoder is never going to run there. Done means doctor lists the tools THIS repository will use.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder doctor` names every PHP tool it looked for, with a version when present and an install line when absent.

## Evidence

Run end to end in a PHP project carrying phpstan.neon and phpcs.xml: `procoder doctor` listed php (PHP 8.5.6), phpcs (4.0.4), phpstan (2.2.8) and prettier (3.9.6) against `.php`. The selection is conditional — lint.HasPhpstanConfig and lint.HasPhpcsConfig gate the two linters, so a repo that chose one is not told to install the other.
