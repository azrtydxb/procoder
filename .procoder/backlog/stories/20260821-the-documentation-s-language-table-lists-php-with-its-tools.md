# The documentation's language table lists PHP with its tools, and the docs gate passes.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A reader deciding whether procoder covers their stack looks at the language table. Done means PHP is in it, with what runs and what happens when nothing is configured.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The documentation's language table lists PHP with its tools, and the docs gate passes.

## Evidence

docs/domains.md gained a PHP row in the lint table naming phpstan and phpcs and the `php -l` floor, and the formatter paragraph now names prettier with @prettier/plugin-php and the out-of-scope behaviour; docs/commands.md matches. `procoder check` reports 27 clean, 0 unformatted, 0 blocking, with the docs obligation satisfied.
