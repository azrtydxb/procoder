# A cache directory that does not exist produces a plain statement and exit 0, not an error.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

procoder may be installed from a release binary rather than the
marketplace. A cache that was never created is not a problem to report,
and reporting it as one would send people looking for a fault they do not
have.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A cache directory that does not exist produces a plain statement and exit 0, not an error.

## Evidence

`TestAMissingCacheDirectoryIsNotAnError` — Compute returns a plan with
nothing removable and no error. Killed by removing the `os.IsNotExist`
branch.

`TestNothingToRemoveSaysSo` covers the neighbouring case: a cache holding
only the active version says "nothing to remove" rather than printing an
empty list that reads like a failure. Killed by removing that branch.

`TestASoleCachedVersionIsLeftAlone` and
`TestAnUnrecognisedDirectoryIsKeptAndNamed` cover the remaining edges — an
unrankable directory is kept AND named, because a person cannot audit what
they are not told.
