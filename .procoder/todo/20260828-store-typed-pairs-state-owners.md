# store: typed pairs for the five state owners, and migrate them

Status: open
Created: 2026-08-28

## Description

Plan task 5 of .procoder/plans/service-state-seam.md.

The five gitignored state owners are where the race actually bites:
dispatch.json, claims.json, env.json, learn.jsonl and the handoff files
in internal/hook/stop.go. Each gets a typed load/save pair on top of the
lock and the atomic write, and each owning package loses its direct
filesystem calls without changing an exported signature.

Reads take no lock — the atomic rename means a reader always sees a whole
file, which is the property that matters, and read locking is out of
scope in the spec.

Done means two concurrent appends to learn.jsonl both survive, which they
do not today.

## Acceptance criteria

- [ ] `TestSaveDispatchLocksAndWritesAtomically` — while a lock on `.procoder/state/dispatch.json` is held, `SaveDispatch` errors naming that path rather than writing.
- [ ] `TestConcurrentAppendsBothSurvive` — 20 concurrent `AppendLearn` calls all appear in the file; removing the `Lock` call makes it fail.
- [ ] All twelve pairs exist: dispatch, claims, env state, learn append/load, handoff, and `LoadMarker`/`SaveMarker` for `last-decisions-digest` and `last-unasked-decision`.
- [ ] `internal/dispatch`, `internal/claims`, `internal/envsync`, `internal/learn` and `internal/hook/stop.go` make no direct filesystem call under `.procoder/`, and their exported signatures are unchanged.
- [ ] A broken stale lock is reported through the command's existing output, not swallowed and not given a new channel.
- [ ] `go test ./...` passes with no change to any existing test.
- [ ] `procoder check` is clean.

## Evidence

