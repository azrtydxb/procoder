# inner-loop

Status: draft

## Problem

The inner loop an agent actually works in is "change one thing, run one
test, then run the app to look at it". procoder covers neither half.
`procoder test` runs whole suites only, so proving a two-line fix means
waiting on every package in the repo — or improvising `go test -run`
by hand and losing the honest reporting that made the command worth
having. And nothing in procoder answers "how do I start this project?":
the agent greps package.json, guesses at a Makefile target, and invents
a command with no evidence behind it. Both are daily, both are
guesswork today, and both are exactly the kind of detection procoder
already does well everywhere else.

## Users

- The agent mid-fix, which needs to run one test by name in whatever
  ecosystem it landed in, and get the same honest verdict back.
- The agent asked to "run the app", which needs the launch command plus
  the evidence for it, so it can start a server in its own background
  shell instead of hallucinating a port.
- Pascal, who wants a one-shot CLI or script executed directly when
  there is exactly one unambiguous candidate, and never wants procoder
  babysitting a long-running server process.

## In scope

- `procoder test --name <pattern>` — narrow the run to matching tests.
  Per ecosystem: Go `-run <pattern>`, pytest `-k <pattern>`, cargo
  `test <pattern>`, gradle `--tests <pattern>`, maven
  `-Dtest=<pattern>`, and the JS test script with the pattern appended
  after `--` (jest and vitest differ on `-t` semantics, so the pattern
  is passed through unchanged and the output says filtering is
  delegated to the runner).
- Any ecosystem whose runner cannot express filtering reports NOT
  filtered — named in the output, never silently running the whole
  suite while wearing a filtered label.
- `--name` composes with the existing `[paths...]` narrowing and with
  `--coverage`; both keep their current meaning.
- `procoder run` — detect and PRINT the launch command(s) for this
  repository, each with the evidence that declared it (the file and the
  line number), ranked most-specific first. Detection sources:
  package.json `scripts` (dev, start, serve — in that order), Makefile
  targets (run, dev, start), go.mod plus a main package
  (`go run ./<dir>`), Cargo.toml `[[bin]]` or src/main.rs
  (`cargo run`), Python (manage.py → `python manage.py runserver`,
  `__main__.py`, pyproject `[project.scripts]`), docker-compose.yml
  (`docker compose up`), Procfile.
- `procoder run --exec` — execute the command, but only when exactly
  one candidate was found and it is not a detected long-running server.
  120s timeout with the hung-tool message. For CLIs, scripts, and
  one-shot commands.
- The long-running heuristic: a command naming a known server verb
  (serve, runserver, dev, start, up, watch) refuses `--exec`, prints
  the command, and tells the agent to run it in its own background
  shell where log capture belongs.
- A repository with no launch candidate says so plainly and exits 0 — a
  library has nothing to run, and that is not a failure.

## Out of scope

- Process management of any kind: no backgrounding, no PID files, no
  log capture, no restart-on-change, no port probing or health checks.
  A server's lifetime belongs to the agent's own shell.
- Guessing a command from nothing: with no evidence there is no
  candidate, ever. `run` never invents a plausible-looking command.
- Environment setup — venv activation, `npm install`, `.env` loading,
  docker builds. `run` prints what the repository declared; the
  prerequisites are the repository's business.
- `--exec` on multiple candidates, or any interactive picker.
- Test filtering by file, tag, marker, or regex translation between
  ecosystems: the pattern is passed to each runner in that runner's own
  syntax, untranslated.
- Watch mode for `test`.

## Constraints

- Pure Go stdlib. Part A extends internal/testrun; Part B is a new
  package internal/runcmd (detection and execution both).
- P-CONTROL: `run` detects and prints — the default path writes
  nothing and executes nothing. `--exec` is an explicit opt-in that
  runs a command the operator asked for; it is never a write to the
  user's code or files.
- Honesty rule, three distinct answers that must never collapse into
  each other: NOT filtered (the runner ran, unfiltered, and said so),
  NOT run (the runner did not execute at all), and no candidates (`run`
  found nothing to launch).
- Every path printed by either command uses forward slashes, on every
  platform — a Windows CI leg asserts this.
- `--exec` timeout is 120s; on expiry the standard hung-tool message,
  and the result is NOT run, never a pass.
- Evidence is mandatory: a candidate without a file and line to point
  at is not a candidate and is not printed.

## Interfaces

- `procoder test [--coverage] [--name <pattern>] [paths...]`. Exit
  codes unchanged: 0 all pass, 1 any fail, 2 nothing could run.
- `internal/testrun`: `Run(root string, paths []string, coverage bool,
name string) []Result` — `name` empty means today's behaviour. A new
  `Result.Filtered bool` records whether the filter reached the runner;
  false with a non-empty `name` prints "NOT filtered" in the Detail.
  `Suite` keeps its no-filter signature.
- `procoder run [--exec]`. Exit 0 when candidates were printed or none
  exist, 0 when `--exec` succeeded, 1 when `--exec` ran and the command
  failed or timed out, 2 when `--exec` was refused (several candidates,
  or a long-running server) or on usage error.
- `internal/runcmd`: `type Candidate struct {Command, Source string;
Line int; LongRunning bool}`; `func Detect(root string) []Candidate`
  ranked most-specific first; `func Report(cands []Candidate, exec
bool, out func(string)) int`.
- Output shape, one candidate per line: the command, then its evidence
  as `source:line` with forward slashes, e.g.
  `go run ./cmd/procoder    (go.mod:1, main package cmd/procoder)`.
  Long-running candidates carry a marker plus the run-it-yourself note.
- Usage text, docs.Commands, the docs site, and a commands/run.md skill
  with its OpenCode twin; commands/test.md gains `--name`.

## Data

- No stored state, no config keys. Both commands read the repository's
  own declarations (package.json, Makefile, go.mod, Cargo.toml,
  pyproject.toml, docker-compose.yml, Procfile) and print; nothing is
  cached, nothing is written under `.procoder/`.

## Edge cases

- `--name` matching zero tests: Go and pytest exit 0 with no tests run.
  That reports pass with "0 test(s) matched <pattern>" — honest, not a
  silent green implying the suite ran.
- `--name` across several ecosystems: each applies the filter in its
  own syntax; an ecosystem that cannot filter reports NOT filtered
  while the others report filtered, in the same run.
- A pattern with shell metacharacters or spaces: passed as one argv
  element, never through a shell.
- package.json with both `dev` and `start`: both are candidates, `dev`
  ranked first; `--exec` therefore refuses (more than one).
- A Makefile `run` target and a package.json `start`: two candidates,
  the more specific source (an explicit `run` target) ranked first,
  `--exec` refused.
- Go module with several main packages: each is its own candidate, all
  printed, `--exec` refused.
- Go module that is a library (no main package): contributes no
  candidate; the repo may still have none at all and say so.
- A Makefile target named `run` whose recipe is `docker compose up`:
  long-running by the verb in the recipe as well as the target name.
- Procfile with several process types: web first, then the rest.
- pyproject `[project.scripts]` with multiple entry points: each is a
  candidate, none is guessed to be "the" one.
- `--exec` on a command that reads stdin: stdin is closed, so it fails
  fast rather than hanging until the 120s timeout.

## Failure modes

- The runner binary is missing with `--name` given: NOT run for that
  ecosystem, exactly as today — filtering does not change detection.
- A malformed package.json or pyproject.toml: that source contributes
  no candidates and says it was unreadable, rather than being silently
  skipped; other sources still report.
- Makefile present but unparseable (includes, generated targets): the
  targets that could be read are reported, and the file is noted as
  partially parsed.
- `--exec` and the command's binary is not installed: exit 1 naming the
  missing binary; the candidate and its evidence are still printed so
  the agent can run it after installing.
- `--exec` on a command that hangs: killed at 120s, hung-tool message,
  NOT run, exit 1.
- docker-compose.yml present but docker absent: still a candidate (the
  repository declared it); `--exec` on it is refused by the
  long-running rule before docker's absence ever matters.

## Acceptance criteria

- [ ] `procoder test --name <pattern>` on a Go fixture with two test
      functions runs only the matching one, verified by the reported
      counts, and exits 0.
- [ ] On a fixture whose ecosystem cannot express filtering, the same
      command reports "NOT filtered" for that ecosystem while the run
      itself still reports its real verdict.
- [ ] The JS path appends the pattern after `--` and the output states
      that filtering is delegated to the runner, pinned by a unit test
      over the constructed argv.
- [ ] `--name` combined with `[paths...]` and `--coverage` in one
      invocation produces both the narrowed package list and a coverage
      number, pinned by a test over the constructed argv per ecosystem.
- [ ] `procoder run` on a fixture with package.json `dev` and `start`
      plus a Makefile `run` target prints all three candidates, each
      with `source:line` evidence, most-specific first.
- [ ] `procoder run` in a repository with no launch declaration prints
      the no-candidates line and exits 0.
- [ ] `procoder run --exec` with two or more candidates refuses and
      exits 2; with exactly one non-server candidate it executes the
      command and exits with 0 on success, 1 on failure.
- [ ] `procoder run --exec` on a single candidate naming a server verb
      (`npm run dev`) refuses, exits 2, and prints the command with the
      instruction to run it in the agent's own background shell.
- [ ] Every path in both commands' output uses forward slashes, pinned
      by a test that rejects a backslash anywhere in the rendered
      output.
- [ ] Usage lists `run`; docs.Commands, the docs site, commands/run.md
      and its OpenCode twin exist, and commands/test.md documents
      `--name`; all rot-guard tests pass.

## Open questions

<!-- none — decisions recorded above -->
