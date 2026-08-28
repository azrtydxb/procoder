# store: typed pairs for the five state owners, and migrate them

Status: closed 2026-08-28
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

- [x] `TestSaveDispatchLocksAndWritesAtomically` — while a lock on `.procoder/state/dispatch.json` is held, `SaveDispatch` errors naming that path rather than writing.
- [x] `TestConcurrentAppendsBothSurvive` — 20 concurrent `AppendLearn` calls all appear in the file; removing the `Lock` call makes it fail.
- [x] All twelve pairs exist: dispatch, claims, env state, learn append/load, handoff, and `LoadMarker`/`SaveMarker` for `last-decisions-digest` and `last-unasked-decision`.
- [x] `internal/dispatch`, `internal/claims`, `internal/envsync`, `internal/learn` and `internal/hook/stop.go` make no direct filesystem call under `.procoder/`, and their exported signatures are unchanged.
- [x] A broken stale lock is reported through the command's existing output, not swallowed and not given a new channel.
- [x] `go test ./...` passes with no change to any existing test.
- [x] `procoder check` is clean.

## Evidence

- `internal/store/state.go` and `internal/store/state_test.go`, plus the
  migration of internal/dispatch, internal/claims, internal/envsync,
  internal/learn and internal/hook. Committed as 3ff036d. `go test ./...`
  green; `procoder check` clean.
- Tests: TestSaveDispatchLocksAndWritesAtomically,
  TestConcurrentAppendsBothSurvive, TestLoadsDoNotLock,
  TestAbsentStateReadsAsAbsent, TestMarkerNameIsNotAPath.
- Mutation-checked: removing the Lock from AppendLearn leaves 1 line of 20;
  treating a newborn empty lock as stale leaves 11 of 20 AND fails
  TestNewbornLockIsNotStolen.
- TestConcurrentAppendsBothSurvive found a second, deeper bug — the
  newborn-lock steal in task 1's lockfile. See the correction appended to
  that task's record. The lock alone was not enough; six appends were
  still lost until staleness was decided by mtime.
- The payload is bytes, not each owner's type: a store returning
  []dispatch.Wave would import dispatch, which imports the store. The spec's
  Interfaces section was corrected to show the byte form and say why.
- Two things the plan's enumeration missed, both now done: learn's
  applied-proposal marker (`.procoder/state/learn-applied.json`) was not
  listed, and `blockedPath` in internal/hook fell out as dead code and is
  deleted.
- envsync had hand-rolled its own temp-and-rename; that is now the store's,
  which also takes the file lock it never had.
- KNOWN, deliberate: `LoadMarker`/`SaveMarker` refuse a name containing a
  separator. Nothing passes a hostile name today, but joining an unchecked
  name would let ".." walk out of .procoder/state.

