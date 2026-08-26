# Over a fixture cache of several versions, `procoder prune` exits 0, names the removable set, and every directory still exists afterwards.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

Somebody typing `procoder prune` to find out what it does must not lose a
gigabyte discovering the answer. Deletion is the one thing here that
cannot be undone, so it is the one thing that is not the default.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Over a fixture cache of several versions, `procoder prune` exits 0, names the removable set, and every directory still exists afterwards.

## Evidence

Two levels, because they can fail independently.

`TestComputingAPlanRemovesNothing` proves the domain: Compute reads and
measures, and every fixture directory is still there afterwards. Killed by
adding an `os.RemoveAll` to Compute's loop.

`TestBarePruneReportsAndRemovesNothing` proves the COMMAND, which is what
a person types. Killed by making `pruneCmd` call Apply regardless of the
flag.

Live against a throwaway fixture: report listed 3 removable, all six
directories present afterwards.
