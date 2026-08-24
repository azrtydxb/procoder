# The corrective loop — fix, prove, re-run whole, until nothing new

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: -

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

- [ ] Every finding has a fix and a regression test that fails without
      it, and a final full run of both passes produces no finding that
      was not already recorded and fixed.
- [ ] Each regression test is proved by running it against the tree
      without the fix and observing it fail.
- [ ] Every finding deferred rather than fixed is filed as an issue and
      named in the final report.
- [ ] The gate is green on this repository at every commit the loop
      produces.

## Evidence
