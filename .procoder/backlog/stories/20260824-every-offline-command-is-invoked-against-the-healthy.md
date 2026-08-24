# The clean pass — every offline command against healthy code

Status: done 2026-08-24
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 014-a-fixture-that-is-not-this-repository-and-a-clean-pass-over

## Description

The first question a new adopter asks procoder is "is my repository
okay", and the worst possible answer is a finding they cannot act on
because the code was already correct. This pass asks that question 78
times, once per command arm in the dispatch, against a fixture built to
be clean.

A finding here is a procoder defect and is recorded with the command
that raised it. So is a crash, a panic, a usage error on documented
flags, and a command that produces no output at all.

The pass records three verdicts, never two. A command whose tool is not
installed on this machine is NOT RUN with the tool named — not a pass,
and not a procoder defect either, because the gap is the machine. The
campaign refusing to distinguish those would be the campaign committing
the failure it is hunting.

## Acceptance criteria

- [x] Every offline command is invoked against the healthy fixture and
      its verdict recorded; any finding raised against correct code is
      reported as a procoder defect with the command that raised it.
- [x] The recorded verdicts are PASS, FINDING or NOT RUN, and every NOT
      RUN names why it could not run.
- [x] The count of commands invoked is reconciled against the dispatch
      arms in `cmd/procoder/main.go`; any arm not invoked is listed with
      the reason.

## Evidence

- `scripts/e2e-clean-pass.sh` runs fifty-three invocations against the
  healthy fixture and records each: **40 pass, 4 finding, 9 NOT RUN**.
  Per-command logs under `$TMPDIR/procoder-e2e-clean/log`.
- Seven procoder defects were raised against correct code and each is
  named with the command that raised it, in
  `.procoder/analysis/e2e-campaign-report.md`. Six are fixed with
  regression tests; F-8 is a question about a documented exit code and is
  recorded rather than changed.
- The three verdicts are distinct and NOT RUN always names the reason —
  eleven absent tools by name, an npm and a python manifest with no lock
  file, an index that has not been built. The classifier claims NOT RUN
  only in procoder's own no-silent-green vocabulary; its first version
  matched the word "missing" and read a finding about an absent PR
  template as a check that never happened, which is recorded above as a
  defect in the campaign rather than in procoder.
- Reconciled against `cmd/procoder/main.go`: forty-five top-level arms,
  every one invoked. The subcommand arms not invoked are listed in the
  report — `hook` (sprint 016), `release` and `copilot-leak` (sprint
  017), and the mutating arms of `backlog`, `sprint`, `todo`, `spec`,
  `plan` and `adr`, which write into the fixture and belong with the
  broken pass.
