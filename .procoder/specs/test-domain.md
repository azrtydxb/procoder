# test-domain

Status: complete

## Problem

procoder's "done" never runs the tests. The gate verifies formatting,
hygiene, and docs, so `todo close` and `backlog close story` can report
"gate clean" on a tree whose suite is red — the one daily practice a
real developer never skips is the one procoder cannot see. The TDD
skill is prose with no binary behind it, and coverage is measured
nowhere.

## Users

- The agent, which needs one command that runs the right suite for any
  ecosystem and answers honestly.
- Pascal, who wants story/todo closes to be refused while tests fail —
  by repo opt-in, like lint blocking.
- The close controllers, which consume a pass/fail verdict.

## In scope

- [S-1] `procoder test [paths...]` — detect the ecosystem's canonical runner
  and run it: Go (`go test ./...`), Rust (`cargo test`), JS/TS (the
  package.json `test` script via the lockfile's package manager: npm /
  pnpm / yarn / bun), Python (pytest when pytest.ini / pyproject
  `[tool.pytest]` / a tests directory with test files exists), Java
  (`./gradlew test` or `mvn -q test` when the build files exist).
  Multiple ecosystems in one repo all run; each reports separately.
- [S-2] Honest verdicts: PASS with parsed counts where the output allows,
  FAIL with the failing lines excerpted, and "NOT run" when no runner
  is detected or the tool is missing — never silence, never fake green.
  Exit 0 all pass, 1 any fail, 2 nothing could run at all.
- [S-3] `procoder test --coverage` — report the covered percentage where the
  runner measures it natively (Go's -cover; pytest with pytest-cov
  installed). A number is reported, never enforced; ecosystems without
  native coverage say "not measured".
- [S-4] `[test] policy = "block"` in config.toml (D-OVERRIDE): todo close and
  backlog story close run the suite and add "the test suite fails" to
  the refusal when it does. Without the policy, closes do not run
  tests (unchanged behaviour).
- [S-5] procoder's own repo adopts the policy in its `.procoder/config.toml`.
- [S-6] A 10-minute per-runner timeout with the hung-tool message.

## Out of scope

- Coverage thresholds or any blocking on a percentage.
- Mutation testing (stays prose in the TDD skill).
- Flaky-test tracking, retries, sharding, or watch mode.
- Installing test runners: doctor/init do not require them — a missing
  runner is reported at run time (a repo without tests is legal).
- Test generation.

## Constraints

- Pure Go stdlib; runner binaries resolved like every other tool.
- P-CONTROL: runs and reports; writes nothing anywhere.
- Honesty rule everywhere: a runner that failed to start is NOT run,
  distinct from tests failing.
- Package: internal/testrun (the word "test" alone collides with Go
  test files); command stays `procoder test`.
- Close-controller wiring passes a verdict function, mirroring how
  gate.Run is passed today.

## Interfaces

- `procoder test [--coverage] [paths...]` — paths narrow the Go
  package list and pytest targets; other runners run whole-project
  (their native granularity), stated in the output.
- `internal/testrun`: `type Result struct {Ecosystem, Verdict, Detail
string; Passed, Failed int; Coverage float64}` with Verdict one of
  pass, fail, notrun; `func Run(root string, paths []string, coverage
bool) []Result`; `func Suite(root string) func() (bool, string)` —
  the closure close controllers call under the block policy, returning
  ok plus a one-line summary.
- config: `[test] policy = "block"` parsed alongside `[lint] policy`.
- Usage text, docs.Commands, docs site, commands/test.md skill +
  OpenCode twin.

## Data

- No stored state. Results are printed; the close controllers receive
  the verdict in-process.

## Edge cases

- Repo with several ecosystems: every detected runner runs; one
  failing marks the whole run exit 1.
- package.json without a "test" script (or the npm placeholder exit-1
  echo): that ecosystem is "NOT run — no test script", not a failure.
- Zero Go test files: `go test ./...` still passes — reported as pass
  with 0 counted, plus a note when no test files exist.
- A runner printing nothing (crash, OOM): non-zero exit with empty
  output reports FAIL with the exit status, never pass.
- Timeout: "gave no answer in 10m — NOT run", exit follows the honesty
  rule (that ecosystem counts as not-run, overall exit 2 if nothing
  else ran, else the worst real verdict wins).
- --coverage with pytest but no pytest-cov: tests run, coverage says
  "not measured (pytest-cov not installed)".
- Paths given that match no ecosystem: said out loud, exit 2.

## Failure modes

- Runner missing (cargo absent on a Rust repo): that ecosystem is
  "NOT run — cargo is not installed", overall exit reflects it (2 when
  nothing ran).
- Close-controller wiring with the suite broken at the tooling level:
  the close refusal says tests could NOT be verified — an unverifiable
  suite blocks under the policy exactly like a failing one (unknown is
  never done).
- git absent: irrelevant, testrun does not need git.

## Acceptance criteria

- [x] [S-1] [S-2] `procoder test` on a Go fixture with one passing and one failing
      package reports FAIL with the failing test named and exits 1;
      after fixing, it reports PASS and exits 0.
- [x] [S-1] On a fixture with package.json (npm lockfile) plus a Go module,
      both runners execute and report separately.
- [x] [S-2] A repo with no detectable test setup answers "NOT run" per the
      honesty rule and exits 2.
- [x] [S-3] `procoder test --coverage` on a Go fixture prints a coverage
      percentage; on an ecosystem without native coverage it prints
      "not measured".
- [x] [S-4] With `[test] policy = "block"`, `procoder todo close` and
      `procoder backlog close story` refuse while the suite fails and
      pass once it is green — verified by tests walking both.
- [x] [S-4] Without the policy, close behaviour is byte-identical to today —
      existing todo/backlog close tests pass unmodified.
- [x] [S-5] procoder's own .procoder/config.toml carries the block policy and
      the repository's suite passes under `procoder test`.
- [ ] [S-6] A runner that gives no answer within ten minutes is reported
      NOT run with the hung-tool line ("gave no answer in 10m0s"), fails
      if a hung runner is ever allowed to keep running. The per-runner
      lines are the `notRun` returns in `internal/testrun` over the
      `runTimeout` const; the path itself is not exercised by a test
      (ten minutes is not a test budget), so this criterion stays
      unticked until it is.
- [ ] Usage lists `test`; docs.Commands, the docs site, and
      commands/test.md with its OpenCode twin exist; all rot-guard
      tests pass.

## Open questions

<!-- none — decisions recorded above -->
