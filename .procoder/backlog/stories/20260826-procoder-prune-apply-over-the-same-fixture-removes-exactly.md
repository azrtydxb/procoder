# `procoder prune --apply` over the same fixture removes exactly the set the report named, and no other directory.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

The report and the sweep must not be able to disagree — `prune` naming one
set and `--apply` removing another is the failure that makes a report
worthless.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder prune --apply` over the same fixture removes exactly the set the report named, and no other directory.

## Evidence

`TestApplyRemovesExactlyThePlanAndKeepsTheWindow` asserts the removed set
by name, not by count, and checks the survivors individually.

There is no second implementation to drift: Apply iterates `plan.Removable`
from the same value the report rendered.

Live: report named 2.0.0, 1.1.0, 1.0.0; `--apply` removed exactly those
three and left 3.1.0, 3.0.0 and the unrecognised directory.
