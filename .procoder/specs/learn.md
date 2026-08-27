# learn

Status: draft

## Problem

Every repository procoder governs runs the same defaults. `procoder bench`
measures the Go code it is pointed at; nothing measures procoder's own
governance overhead. So the question an adopter actually asks — "is this
gate paying for itself in my repository?" — has no answer except opinion,
and the honest position stated in `docs/honest-limits.md` is that no
benchmark of the gate's cost against defects caught has ever been run.

That is uncomfortable for a tool whose entire argument is that claims
should be checkable. It also has a practical cost: a repository whose
Python never trips ruff still pays the ruff pass on every commit, and
nobody can tell whether relaxing it would lose anything, so nobody relaxes
it.

## Users

- **An adopter deciding whether to keep procoder** — needs a number about
  their own repository, not procoder's marketing.
- **A maintainer tuning `.procoder/config.toml`** — needs to know which
  policy is expensive here and whether loosening it costs anything.
- **An agent in a long session** — needs to know which step it is about to
  pay for, so it can say so rather than silently routing around it.

## In scope

- [S-1] Record what a procoder command cost: which command, which
  subcommand or domain, wall-clock duration, exit code, and whether the
  run was blocking. Append-only, one line per run.
- [S-2] learn measure — read those records and rank commands
  and domains by total time and by how often they blocked.
- [S-3] learn propose — print concrete `.procoder/config.toml`
  changes, each carrying the measurement that motivates it and the
  evidence class of that measurement.
- [S-4] Label every finding `measured` or `inferred`, reusing the existing
  `internal/evidence` classification rather than inventing a second one.
- [S-5] `procoder learn verify` — after a proposal has been applied, report
  whether the cost it targeted actually fell, and print the revert when it
  did not.
- [S-6] Opt-in recording: `[learn] record = true`, defaulting to false, so
  no repository starts writing timing data because it upgraded.

## Out of scope

- Applying config changes. The binary prints; the agent writes. `learn`
  proposes a diff and never edits `.procoder/config.toml`, which means the
  issue's "applies only on explicit consent" and "reverts automatically"
  become "prints the change" and "prints the revert". See Constraints.
- Proposing changes to specs, plans, or templates. Config only in this
  version; a tool that rewrites your process based on ten samples is worse
  than no tool.
- Cross-repository aggregation or any network call. The measurements never
  leave the machine.
- Measuring anything about the code procoder governs. This measures
  procoder, not your test suite.

## Constraints

- **P-CONTROL.** The binary must not modify repository content. This is the
  constraint the issue's design did not account for: a closed loop that
  applies and reverts on its own cannot exist here. The loop closes with a
  human or agent in it, and `verify` is what makes that loop honest rather
  than optional.
- **The gate runs on every commit.** Recording must cost effectively
  nothing and must never be able to fail a run that would otherwise pass. A
  write error is dropped silently — this is the one place in procoder where
  a failure is not reported, because reporting it would make measurement
  able to break the thing it measures.
- **Timing data is not repository content.** It goes under
  `.procoder/state/`, which is already gitignored, and is never committed.
- **No new dependency**, per the build principles.
- **Sample size is a first-class output**, not a footnote. A ranking from
  four runs must say it came from four runs.
- **Every run is recorded**, not a sample. Rotation by line count already
  bounds the file, so sampling would trade accuracy for a limit that is
  already enforced — and on a low-traffic repository a one-in-N sample may
  never reach `min_samples` at all. One appended line is negligible beside
  the work being measured.

## Interfaces

- `procoder learn` — the bare form is `procoder learn measure`.
- `procoder learn measure [--since <dur>]` — the ranked cost report.
- `procoder learn propose` — the ranked report plus printed config changes.
- `procoder learn verify` — did the last applied proposal help.
- `[learn] record = true|false` in `.procoder/config.toml` (default false).
- `[learn] min_samples = <n>` (default 20) — below this, `propose` prints
  nothing and says why.

## Data

`.procoder/state/learn.jsonl`, append-only, one JSON object per run:

```
{"cmd":"check","domain":"lint","ms":812,"exit":0,"blocking":false,"at":"2026-08-27T10:00:00Z"}
```

`.procoder/state/learn-applied.json` records which proposal was applied and
when — written by the agent when it applies one, read by the binary. That
is the anchor `procoder learn verify` measures against. Inferring it from
the git history of `.procoder/config.toml` was considered and rejected:
history shows that the file changed, never which proposal a change
corresponds to, nor whether an edit was a proposal at all. A marker can be
forgotten, and `verify` says so rather than guessing.

Both files are rotated by line count, oldest dropped, so an old repository
does not carry an unbounded file. Nothing in it identifies a person, a path outside the
repository, or the content of any file.

## Edge cases

- **Fewer runs than `min_samples`** — `measure` reports what it has and
  names the count; `propose` declines and says how many more it needs.
- **A clock that moves backwards** — a negative duration is dropped, not
  clamped, and the drop is counted in the report.
- **The file is corrupt or partly written** — unparseable lines are skipped
  and counted, and the report names how many. A truncated tail is expected
  from a killed process, not an error.
- **The file cannot be read** — `measure` reports NOT measured and exits
  non-zero. It never reports "no cost" for data it could not read.
- **Recording disabled the whole time** — `measure` says recording is off
  and names the setting, rather than reporting an empty ranking.
- **A command that never finishes** — no record is written, so the ranking
  under-counts. `measure` says so where the count of starts exceeds ends.

## Failure modes

- **Recording fails to write** — dropped silently, per Constraints. The
  measured command's own result is untouched.
- **`.procoder/state/` does not exist** — created on first record; if
  creation fails, recording is off for that run and nothing else changes.
- **A proposal is applied and the cost rises** — `verify` says so and
  prints the revert. It never applies it.
- **Two sessions record concurrently** — appends are line-oriented and
  opened in append mode; an interleaved partial line is handled by the
  corrupt-line path above rather than by locking.

## Acceptance criteria

<!-- Each criterion names the test that asserts it; each test carries its
     own `proved by:` mutation, per the build principles. -->

- [ ] [S-1] `TestARecordIsAppendedOnlyWhenRecordingIsOn` asserts exactly one
      JSONL line is appended per run once recording is enabled in
      `.procoder/config.toml`, and that no file exists at all by default.
- [ ] [S-1] `TestARecordWriteFailureChangesNothing` asserts a failing record
      write leaves the measured command's exit code and output untouched.
- [ ] [S-2] `TestMeasureRanksDomainsByTotalDuration` asserts the ranking over
      a fixture of known records, and that the sample count is printed.
- [ ] [S-2] `TestMeasureOnAnUnreadableFileSaysNotMeasured` asserts NOT
      measured and a non-zero exit, never an empty ranking.
- [ ] [S-2] `TestMeasureWithRecordingOffNamesTheSetting` asserts the
      recording setting is named rather than an empty result being printed.
- [ ] [S-2] `TestCorruptAndNegativeLinesAreCountedNotDropped` asserts corrupt
      and negative-duration lines are skipped, counted, and that both counts
      appear in the report.
- [ ] [S-3] `TestProposePrintsAndWritesNothing` asserts
      `.procoder/config.toml` is byte-identical after a proposal is printed.
- [ ] [S-3] `TestProposeBelowMinSamplesDeclines` asserts no proposal is
      printed and that the shortfall in runs is named.
- [ ] [S-4] `TestEveryNumberCarriesItsEvidenceClass` asserts every printed
      line carrying a number is labelled measured or inferred, and that the
      labels come from `internal/evidence`.
- [ ] [S-5] `TestVerifyPrintsTheRevertWhenCostDidNotFall` asserts the
      direction of change is reported against the pre-proposal records, and
      that a revert is printed when the cost did not fall.
- [ ] [S-6] `TestRecordingIsOffByDefault` asserts that with no `[learn]`
      section, `procoder check` creates no `.procoder/state/learn.jsonl`.
- [ ] [S-7] `TestALooseningProposalStatesWhatItCannotSee` asserts every
      proposal that downgrades a blocking policy carries the line naming
      the defects the measurement cannot account for.
- [ ] [S-5] `TestVerifyWithoutAMarkerSaysSo` asserts that with no
      `.procoder/state/learn-applied.json`, verify reports it has no
      anchor rather than inferring one.

## Open questions

<!-- All three resolved with the user on 2026-08-27 and rewritten as
     decisions above: recording frequency in Constraints, the loosening
     rule as [S-7] in In scope, and the verify anchor in Data.

     Left empty deliberately: any non-empty line in this section counts as
     an open question, "None." included. -->
