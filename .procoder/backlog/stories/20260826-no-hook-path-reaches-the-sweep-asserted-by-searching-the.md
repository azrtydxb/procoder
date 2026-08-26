# No hook path reaches the sweep: asserted by searching the hook entry points for the call, so a later change that wires it in fails this test.

Status: done
Created: 2026-08-26
Epic: plugin-cache-retention
Sprint: 023-updating-in-place-stops-leaving-every-previous-version-behind

## Description

Hooks fire on every write, every commit, every session start. A delete of
this size is a deliberate action a person takes, not something that
happens to them while they are typing.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] No hook path reaches the sweep: asserted by searching the hook entry points for the call, so a later change that wires it in fails this test.

## Evidence

`TestNoHookPathReachesTheSweep` reads every non-test source in
`internal/hook/` and fails if any mentions the package. Asserted by
reading rather than by running: a behavioural test proves only the paths it
happened to exercise, and what is wanted is absence from all of them.

Killed by adding a `plugincache` import to `internal/hook/hook.go` — the
test names the file.

It also guards itself: if no source files are read, it fails rather than
passing on an empty set. Killed separately by pointing it at a package
that does not exist.
