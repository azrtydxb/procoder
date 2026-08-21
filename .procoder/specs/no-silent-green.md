# no-silent-green

Status: COMPLETE

## Problem

The gate reports green when nothing checked the code. Three routes to the
same false verdict, all of them live today.

A linter that is not installed produces `NOT checked — golangci-lint is
not installed`, printed as `info`, and the gate exits 0. Domain 1 already
knows better: a missing gitleaks is BLOCKING. Domain 2 says the same
sentence and shrugs.

A file whose formatter needs a project config it cannot find is reported
out of scope, which the gate counts under "skipped" and passes. So a C or
C++ repository with no `.clang-format`, and a PHP repository without the
prettier plugin, are formatted by nothing and told they are fine.

TypeScript with no eslint config is out of scope by design — the reasoning
was that a parser would have to be imposed. The result is that the most
common TypeScript setup in the world, a repo with a tsconfig and no eslint
config, gets no linting and a green gate.

And several extensions are formatted but reach no linter at all: `.mts`,
`.cts`, `.pyi`, C/C++, C#, and Dart. `.mjs` and `.cjs` lint while their
TypeScript twins do not, which is not a decision anybody made.

## Users

- **Anyone adopting procoder** needs the first green gate to mean the code
  was checked, not that the machine was empty.
- **A team on a language procoder claims to support** needs a working
  default, and their own config to win the moment they write one.
- **A maintainer** needs to be told what is missing and how to install it,
  loudly enough that it gets installed.

## In scope

- A check that could not run is BLOCKING, in every domain, matching the
  rule domain 1 already follows.
- Formatting never reports out of scope for want of a config: clang-format
  gets a procoder baseline style, and a missing prettier PHP plugin is a
  missing tool rather than a style opinion.
- TypeScript with no project config is linted against a procoder baseline.
- Every extension procoder formats reaches a linter, or is told plainly
  that none ran.
- doctor and init know every default tool, so the loud refusal has a
  remedy the same command can carry out.

## Out of scope

- Changing what any tool reports once it runs. This is about checks that
  never happened.
- `[lint] policy = "report"` for real findings. A finding is a judgement
  and stays reportable; "the check did not happen" is not a judgement.
- Data formats — JSON, CSS, Markdown, YAML — which are formatted and have
  no linter by design; YAML workflows already reach actionlint.
- Writing any config into the user's repository. Baselines live in temp
  files or on the command line, as Go's already does.

## Constraints

- **D-OVERRIDE.** A project config always wins over a procoder baseline.
- **No silent green.** Out of scope means procoder does not claim the file
  type, never that it claims it and could not check it.
- Baselines must not shout on existing code: a default that fires on every
  untouched file is a default people turn off.
- No tool procoder installs without being asked.

## Interfaces

- No new command. `procoder check`, `lint`, `format`, `doctor` and `init`
  keep their shapes; what changes is which verdict they give.
- doctor gains rows for the new default tools.

## Data

- No new state. Baselines are temp files or command-line arguments,
  removed or forgotten after the run.

## Edge cases

- A repo with a `.clang-format`: its style wins, procoder's preset unused.
- A repo with an eslint config covering some directories: covered files use
  it, uncovered TypeScript gets the baseline.
- A machine with no tools at all: every domain says so, blocking, and
  `procoder init` installs them.
- A language procoder formats but has no linter for: said plainly, not
  omitted.
- A file type procoder does not claim, such as a text file or an image:
  still out of scope, still silent, still green — that case is legitimate
  and must not be swallowed by this change.

## Failure modes

- A blocking refusal with no remedy is a wall: every refusal names the
  tool and the command that installs it.
- A baseline that cannot be written must not silently degrade to a thinner
  check reported as the full one.
- clang-tidy without a compilation database still analyses a single file;
  a wrong invocation must surface as NOT checked, never as clean.

## Acceptance criteria

- [ ] With no linter installed, `procoder check` over a file of that
      language exits 1 and the NOT-checked line is BLOCKING, not info.
- [ ] A C++ file in a repository with no `.clang-format` is formatted
      against procoder's baseline style and reported clean or unformatted,
      never out of scope.
- [ ] A repository carrying its own `.clang-format` is formatted by that
      file, asserted by a style that differs from the baseline.
- [ ] A PHP file with no prettier plugin is UNCHECKED with the install
      line and fails the gate, where it previously passed as out of scope.
- [ ] A `.ts` file in a repository with no eslint config is linted against
      procoder's baseline and reports a real finding.
- [ ] `.mts` and `.cts` files reach the same linter as `.ts`, and `.pyi`
      reaches ruff, asserted by a fixture of each.
- [ ] A C++ file reaches clang-tidy and a real finding is reported with its
      file and line.
- [ ] A language procoder formats but cannot lint reports NOT checked,
      blocking, naming the language — never nothing.
- [ ] A file type procoder does not claim is still out of scope and still
      passes, asserted so the change cannot swallow every unknown file.
- [ ] `procoder doctor` lists every new default tool with its install line,
      and `procoder init` plans to install them.

## Open questions

<!-- none — decisions below -->

## Decisions

- **D-1: "could not check" is blocking everywhere.** Domain 1 already
  blocks on a missing gitleaks. Domain 2 printing the identical sentence as
  `info` was an inconsistency, not a policy, and it is the single largest
  source of green gates over unchecked code. `[lint] policy` still governs
  findings, because a finding is a judgement; whether the check ran is not.
- **D-2: clang-format gets a baseline style rather than a config
  requirement.** The old reasoning — that a style without a config would be
  imposing LLVM's taste — was right that procoder must not write a config
  into the repo, and wrong that the alternative was checking nothing. A
  named preset passed on the command line imposes nothing on disk and
  formats the file; a repo `.clang-format` still wins.
- **D-3: the missing prettier PHP plugin is a missing TOOL.** It was
  modelled as a missing style config, which made it out of scope and green.
  Nothing about it is a style opinion — it is the thing that parses PHP.
- **D-4: TypeScript gets a baseline through typescript-eslint.** The parser
  is a tool procoder installs and doctor names, exactly like every other
  tool, rather than a reason to check nothing.
- **D-5: C/C++ gets clang-tidy with a curated check set.** Verified to run
  on a single file with no compilation database. The set is the analyser
  and bug-prone families — the classes that are bugs in any codebase — not
  style, which clang-format already owns.
