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

**What slowed us down.** Two tests carried `proved by:` lines naming
mutations that could not fail them: both called a helper directly, so
mutating the call site changed nothing. They were caught only because the
mutations were actually run rather than asserted in a comment. Writing the
comment is not the discipline; running the mutation is.

The sprint also shipped a gap it had just created. phpstan reached the
required-tools list only where a config already existed, so a PHP project
with no tooling was told nothing was missing — the exact opposite of the
intent, closed the same day in a follow-up. A criterion that had said
"doctor installs the default linter" rather than "doctor names every PHP
tool it looked for" would have caught it at seeding time.

**What we change.** A `proved by:` line is not written until the mutation
has been applied and the test seen to fail. And a criterion about a
default names what happens when the project has NOTHING, not what happens
when it already has something — the empty case is the one that ships
broken, because it is the one nobody has locally.

**Worth keeping.** Deciding by measurement rather than taste. The phpstan
level was picked by running levels 0 through 6 against ordinary untyped
legacy PHP and counting: 0 through 5 silent, level 6 four findings on
fourteen lines. That number ended the argument in one command, and it is
in the spec where the next person will find it. Same for the formatter —
php-cs-fixer, phpcbf and pint were each run to see whether they could
print to stdout, rather than reasoned about.
