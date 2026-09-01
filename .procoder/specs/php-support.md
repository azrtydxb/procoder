# php-support

Status: COMPLETE

## Problem

PHP is one of the most widely deployed languages on the web and procoder's
gates do not see it at all. A `.php` file is out of scope for formatting,
invisible to linting, and a repository whose whole suite is phpunit is told
"NOT run — no recognized test setup". The gate reports clean over a tree it
never looked at, which is worse than reporting nothing: a PHP team adopting
procoder gets a green verdict that means only that procoder does not speak
their language. People have asked for it.

## Users

- **A PHP developer** wants the same contract every other language gets: the
  formatter prints the corrected file, the linter names real findings, the
  suite runs, and anything that could not be checked says so.
- **A team on a mixed repo** (PHP plus JS, or PHP plus Go) wants one gate
  with one verdict, not a gate that silently covers half the tree.
- **A maintainer of a PHP project with its own standards** wants their
  `phpstan.neon` or `phpcs.xml` obeyed, not overridden by procoder's taste.

## In scope

- [S-1] Formatting `.php` through prettier's PHP plugin, which prints the
  formatted source to stdout and leaves the file untouched.
- [S-2] Linting `.php`: phpstan when the project carries a phpstan config, phpcs
  when it carries a phpcs config, and `php -l` as the floor when it carries
  neither.
- [S-3] Running phpunit as a detected test runner in `procoder test`, with pass,
  fail, counts and `--name` filtering.
- [S-4] `doctor` and `init` knowing the PHP tools: what is missing, and the line
  that installs it.
- [S-5] Documentation: the language table in the docs gains PHP.

## Out of scope

- Choosing a coding standard for anyone. procoder ships no `phpcs.xml`, no
  `phpstan.neon` and no rule set; a project without config gets the syntax
  floor, never an imposed style.
- php-cs-fixer and Laravel Pint as formatters. Both are excellent and
  neither can print a formatted file to stdout — see Constraints.
- Coverage reporting for phpunit. `procoder test --coverage` will report
  NOT measured for PHP rather than pretend.
- psalm, phan, and other analysers beyond the two configured above.
- Composer dependency freshness in `procoder deps`.

## Constraints

- **P-CONTROL.** The binary prints; the agent writes. A formatter is only
  usable here if it can emit the formatted source on stdout without
  touching the file. This was tested, not assumed: `php-cs-fixer fix -`
  reads stdin but reports which files _can_ be fixed rather than emitting
  the fixed source, and `--diff` emits a diff; `phpcbf` and `pint` write in
  place. Only prettier with `@prettier/plugin-php` prints the result, which
  is what settles the formatter choice on evidence rather than on taste.
- **D-OVERRIDE.** The project's own config always wins. A repository with a
  `phpstan.neon` is linted by its own rules and levels.
- No new Go dependency, and no new tool procoder installs without being
  asked.
- The prettier PHP plugin is a project-local npm package, so a repository
  that does not carry it must be told its `.php` files are out of scope and
  why — counted and said, never silently reported clean.

## Interfaces

- `procoder format <file.php>` prints the formatted source, exactly as it
  does for every other language.
- `procoder lint <file.php>` prints findings as `path:line: message`.
- `procoder test` detects phpunit and reports its counts.
- `procoder check` covers `.php` through the same gate as everything else.
- `procoder doctor` lists the PHP tools, their versions, and what is absent.
- No new command and no new flag.

## Data

- No new state. Detection reads files already on disk: `phpstan.neon`,
  `phpstan.neon.dist`, `phpcs.xml`, `phpcs.xml.dist`, `phpunit.xml`,
  `phpunit.xml.dist`, and `vendor/bin/`.
- The formatter's plugin is found through the project's `node_modules`, the
  same resolution prettier already uses.

## Edge cases

- A `.php` file in a repository with no prettier PHP plugin: out of scope,
  counted and named, with the install line.
- A repository with both `phpstan.neon` and `phpcs.xml`: both run, and the
  findings merge.
- A repository with neither: `php -l` runs, and only real syntax errors are
  reported — a file that parses is clean, not styled.
- No `php` binary at all: NOT checked, naming php, never silently clean.
- `php -l` on a file that parses prints "No syntax errors detected" and
  must produce no finding.
- phpunit present but no test file: the runner reports what phpunit
  reported rather than inventing a pass.
- A `.php` file that is a template with inline HTML still formats; a file
  that fails to parse cannot be formatted and must say so, not emit
  garbage.

## Failure modes

- A linter that exits non-zero with no parseable output is NOT checked with
  the reason, never a silent pass.
- phpstan's raw format prints `path:line:message` with no space after the
  line number, which the existing finding parser does not match; a parser
  that silently drops every phpstan finding would report a clean lint over
  a file full of errors.
- A missing tool is reported as NOT checked, never as clean.
- phpunit writing its failure detail to stdout while exiting 1 must be read
  as a failed suite, not as a runner that could not run.

## Acceptance criteria

- [x] [S-1] `procoder format` on a fixture `.php` file prints the formatted
      source, exits 0, and leaves the file's bytes unchanged — asserted by
      comparing the file's digest before and after.
- [x] [S-1] A repository whose `node_modules` has no `@prettier/plugin-php`
      reports its `.php` files out of scope with the install line, and the
      gate counts them as out of scope rather than clean.
- [x] [S-2] On a fixture carrying `phpstan.neon`, `procoder lint` over a file
      with a wrong return type names the file, the line, and the message,
      parsed from phpstan's raw format including its missing space.
- [x] [S-2] On a fixture carrying `phpcs.xml`, `procoder lint` names a PSR-12
      finding with its file and line.
- [x] [S-2] On a fixture carrying both configs, findings from both tools appear.
- [x] [S-2] On a fixture carrying neither, a file with a syntax error is reported
      with its line, and a file that parses produces no finding at all.
- [x] [S-2] With `php` absent from a stub PATH, `procoder lint` over a `.php`
      file prints a NOT-checked line naming php and never reports clean.
- [x] [S-3] `procoder test` on a fixture with `phpunit.xml` and a passing suite
      reports passed with its test count; the same fixture with one failing
      test reports failed and exits 1.
- [x] [S-3] `procoder test --name <pattern>` passes the pattern to phpunit's
      `--filter` and the reported count reflects the narrowed run.
- [x] [S-3] `procoder test --coverage` on a phpunit project reports coverage NOT
      measured rather than a number.
- [x] [S-4] `procoder doctor` names every PHP tool it looked for, with a version
      when present and an install line when absent.
- [x] [S-5] The documentation's language table lists PHP with its tools, and the
      docs gate passes.

## Open questions

<!-- none — decisions below -->

## Decisions

- **D-1: prettier with `@prettier/plugin-php` is the formatter.** Settled
  by P-CONTROL rather than preference: it is the only PHP formatter tested
  that prints formatted source to stdout and leaves the file alone.
  php-cs-fixer, phpcbf and pint all either write in place or emit a diff,
  and working around that would mean temp files, which the formatter
  registry exists to avoid.
- **D-2: the linter is whichever the project configured; with none,
  procoder brings its own.** phpstan when a phpstan config is present,
  phpcs when a phpcs config is present, both when both. The project's own
  config always wins (D-OVERRIDE).
- **D-5 (revises D-2): an unconfigured project gets procoder's phpstan
  baseline, not a syntax floor.** D-2 originally stopped at `php -l` when
  nothing was configured, on the reasoning that procoder imposes no
  standard. That reasoning was right about style and wrong about bugs: a
  wrong return type and a call to a function that does not exist are not
  matters of taste, and leaving them unreported meant most PHP
  repositories — the ones with no linter configured on the day procoder
  arrives — got a gate that only checked whether the file parsed. procoder
  now supplies a curated phpstan config, written to a temp file so nothing
  lands in the repository, exactly as it already does for Go's golangci
  baseline and Java's bundled google_checks. `php -l` remains as the floor
  for when phpstan itself is absent, alongside a NOT-checked line naming
  it so `procoder init` installs it.
- **D-6: the baseline is phpstan at level 5, and phpstan rather than
  phpcs.** phpcs is a style checker and formatting is already prettier's
  job here, so a phpcs default would put two tools in charge of the same
  thing. The level was measured, not chosen: against ordinary untyped
  legacy PHP, levels 0 through 5 report nothing while level 6 demands a
  typehint on every parameter and produced four findings on fourteen
  lines. A default that shouts at every existing codebase is a default
  people turn off.
- **D-3: phpunit ships in the same change as format and lint.** PHP
  support that stops at the gate leaves `procoder test` telling a PHP team
  it has no recognized test setup, which is the same wrong answer in a
  different domain.
- **D-4: coverage is reported NOT measured for PHP.** phpunit's coverage
  needs xdebug or pcov installed and configured in the PHP runtime;
  claiming a number procoder did not measure is worse than saying it did
  not measure one.
