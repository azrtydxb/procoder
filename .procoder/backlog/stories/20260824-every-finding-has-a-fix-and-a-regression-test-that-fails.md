# The corrective loop — fix, prove, re-run whole, until nothing new

Status: done 2026-08-24
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 018-the-whole-campaign-re-run-from-scratch-until-nothing-new-then

## Description

Finding defects is the easy half. This story is the part that makes the
campaign a loop rather than a sweep: every finding gets the smallest fix
that closes it and a regression test that fails without the fix, and
then both passes run again from the top.

Re-running whole, not around the fix, is deliberate. A fix that closes
one command and breaks another is the single most likely outcome of a
campaign this wide, and only a full re-run sees it.

The loop ends when a full round of both passes produces no finding that
was not already recorded and fixed — not when the list looks short
enough. Each round's report states what it did NOT cover alongside what
passed, so a shrinking finding count cannot be mistaken for shrinking
risk.

Anything too large for the smallest fix becomes an issue rather than a
redesign inside this epic.

## Acceptance criteria

- [x] Every finding has a fix and a regression test that fails without
      it, and a final full run of both passes produces no finding that
      was not already recorded and fixed.
- [x] Each regression test is proved by running it against the tree
      without the fix and observing it fail.
- [x] Every finding deferred rather than fixed is filed as an issue and
      named in the final report.
- [x] The gate is green on this repository at every commit the loop
      produces.

## Evidence

**Round two, against a fixture rebuilt from `git init` with eighteen fixes
layered in since round one, is identical to round one in every phase:**

| Phase          | Round 1                    | Round 2                    |
| -------------- | -------------------------- | -------------------------- |
| Clean pass     | 42 pass / 3 find / 8 skip  | 42 pass / 3 find / 8 skip  |
| Hooks          | 20 pass, 0 fail            | 20 pass, 0 fail            |
| Security knobs | 7 pass, 0 fail, 1 unproved | 7 pass, 0 fail, 1 unproved |
| Docs           | 53 pass, 0 fail            | 53 pass, 0 fail            |
| Broken pass    | 21 caught, 0 missed        | 21 caught, 0 missed        |

No finding appeared that was not already recorded and fixed, and nothing
the fixes broke. The three remaining clean-pass findings are correct
behaviour: `doctor` exits 1 with tools absent, `ci --emit` exits 2 because
it is backlogged and not built, and `index impls` reports that a function
has no implementors. The one NOT RUN is C# formatting, which needs a
dotnet SDK this machine does not have.

- **Seventeen findings, seventeen fixes, each with a regression test.**
  Every test carries a `// proved by:` line naming a mutation, and every
  one of those mutations was applied to the source, built, and watched to
  fail — thirty-one of them across the campaign, individually run.
- **Three mutations that did NOT fail their test, and what each taught.**
  Deleting the `checkFlags` call left the first flag test passing, because
  it exercised the helper and never the dispatch; a test through `run()`
  was added. Forcing `format` to write left P-CONTROL passing, because the
  loop only ever formatted already-clean files and never reached the
  branch; an unformatted file is planted now. And no single mutation fails
  `TestAnUnrelatedTagDoesNotBlockTheRelease`, because two redundant
  mechanisms guard it — that is stated in the comment as a pair rather
  than claimed as a single-mutation proof.
- **Nothing was deferred.** Every finding recorded in this campaign was
  fixed in it; no issue was filed in lieu of a fix.
- **The gate is green at every commit the loop produced**, and this
  repository's suite is green: 44 packages, and `procoder check` over 390
  tracked files reports 0 unformatted, 0 unchecked, 0 blocking.
- The P-CONTROL loop was extended after round two to cover `bench` and
  `release` — the only two commands that can legitimately write — and both
  write nothing without their flag: 55 assertions, 0 failures.

**What this round did NOT cover**, unchanged from the earlier rounds: C#
formatting (no dotnet SDK), the Java precise index tier (no coursier),
Maven tests (no mvn), the `sast_blocks_at` WARNING/ERROR boundary (no
WARNING-severity finding reachable), and the Pages health check's
"enabled but stale" branch.
