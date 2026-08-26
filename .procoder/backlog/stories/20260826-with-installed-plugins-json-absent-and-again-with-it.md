# With `installed_plugins.json` absent, and again with it holding unparseable content, `--apply` exits 2 and every directory still exists.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

Not knowing which version is in use is not a licence to delete. Three
shapes of not knowing, each reaching the failure by a different path.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With `installed_plugins.json` absent, and again with it holding unparseable content, `--apply` exits 2 and every directory still exists.

## Evidence

`TestAnUnreadableRegistryRefusesAndRemovesNothing` covers absent,
unparseable, and procoder-not-listed, asserting in each case that Compute
errors AND that every directory survives. Killed by making the read-error
branch return a nil error.

`TestARefusalExitsTwoAndRemovesNothing` pins the exit code the criterion
names — 2 — at the command surface, over the same three shapes. Killed by
returning 0 from that branch.

Live, with `--apply` in all three cases: exit 2, nothing removed.
