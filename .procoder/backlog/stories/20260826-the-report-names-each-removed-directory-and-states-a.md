# The report names each removed directory and states a reclaimed total that equals the summed size of the directories that actually went — verified against a fixture of known sizes, not against the figure the code computed.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

A sweep nobody can audit is a sweep nobody trusts. And a reclaimed figure
taken from the plan rather than from reality means a sweep that removed
nothing still reports a number.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The report names each removed directory and states a reclaimed total that equals the summed size of the directories that actually went — verified against a fixture of known sizes, not against the figure the code computed.

## Evidence

`TestTheReclaimedFigureIsWhatActuallyWent` uses a fixture of known sizes —
3 KB + 7 KB removable against 5 KB + 5 KB kept — and asserts 10 KB by
arithmetic, not against whatever the code produced. Killed by moving
`reclaimed += size` above the RemoveAll error check.

`TestApplyNamesWhatWentAndWhatCameBack` pins the report naming each removed
directory and stating the total.

A directory that cannot be removed is named with its reason and left out of
the total; the command exits 1 when any failed.
