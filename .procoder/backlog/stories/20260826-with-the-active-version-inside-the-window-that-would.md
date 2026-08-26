# With the active version inside the window that would otherwise be dropped, it survives; and with the running binary's own directory inside that set, it survives. Asserted separately, so one check passing cannot hide the other being absent.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

The unrecoverable failure. Deleting the version in use leaves somebody
with no working install, and the two ways it could happen are independent:
the registry names a version, and a binary executes from a directory.
Either check alone leaves a way through.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With the active version inside the window that would otherwise be dropped, it survives; and with the running binary's own directory inside that set, it survives. Asserted separately, so one check passing cannot hide the other being absent.

## Evidence

Asserted separately, as the criterion requires.

`TestTheActiveVersionSurvivesEvenWhenOld` uses an active version of 1.3.0
with 3.1.0, 3.0.0 and 2.0.1 also cached — the window would drop it, and it
survives. Killed by emptying the `kept` map's seed.

`TestTheRunningDirectorySurvivesTheRegistrySayingOtherwise` sets the
running directory to 1.3.0 while the registry names 3.1.0, so the registry
check cannot be what saves it. Killed by deleting the `sameDir` case.

Also covered: `TestAnAbsentActiveVersionStopsTheSweep` — a registry naming
a version that is not on disk stops the sweep entirely rather than removing
"everything else".
