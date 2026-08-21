# A repository whose `node_modules` has no `@prettier/plugin-php` reports its `.php` files out of scope with the install line, and the gate counts them as out of scope rather than clean.

Status: done 2026-08-21
Created: 2026-08-21
Epic: php-support
Sprint: 008-php-in-the-gates-format-lint-and-phpunit

## Description

A team without the plugin installed must not be told their PHP is fine. Done means the file is counted out of scope, named, and the reader is given the line that installs it rather than a path into node_modules.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A repository whose `node_modules` has no `@prettier/plugin-php` reports its `.php` files out of scope with the install line, and the gate counts them as out of scope rather than clean.

## Evidence

`go test ./internal/format/ -run TestWithoutThePluginPHPIsOutOfScopeWithTheInstallLine`: PASS — verdict OutOfScope and the reason carries `npm i -D`. Proved by mutation: removing the Tool.ConfigMissing field makes the reason "no node_modules/@prettier/plugin-php/src/index.mjs in the project", which names what is absent and nothing about what to do; the test then fails.
