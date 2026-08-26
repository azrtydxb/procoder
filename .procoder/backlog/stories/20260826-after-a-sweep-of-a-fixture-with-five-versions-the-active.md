# After a sweep of a fixture with five versions, the active version and exactly one previous remain.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

Repointing `installed_plugins.json` at the directory below is the only
cheap rollback there is. A window of one leaves none.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] After a sweep of a fixture with five versions, the active version and exactly one previous remain.

## Evidence

`TestApplyRemovesExactlyThePlanAndKeepsTheWindow` over five versions:
3.1.0 (active) and 3.0.0 remain, 2.0.1/1.4.0/1.3.0 go. Killed by
`Keep = 2` → `Keep = 1`.

The window is anchored on the ACTIVE version rather than the newest, so
somebody deliberately running an older build does not have the newest kept
and their own swept — that is what the 1.3.0-active fixture proves.
