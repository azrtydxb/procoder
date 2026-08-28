# store: typed pairs for the twenty content owners, and migrate them

Status: closed 2026-08-28
Created: 2026-08-28

## Description

Plan task 6 of .procoder/plans/service-state-seam.md.

The remaining twenty owners hold repository content rather than session
state: adr, analysis, answers, backlog, bench, codeindex, config, docs,
glossary, gitcmd, lessons, lint, plan, principles, review, security,
spec, templates, todo, wizard.

Directory-backed owners get ListDir/LoadIn/SaveIn; single-file owners get
LoadDoc/SaveDoc. Locking stays per file, not per directory — two people
editing two different stories must not serialise against each other.

Done means no package outside internal/store reads or writes under
.procoder/ directly, and nothing about any file's format or location has
moved.

## Acceptance criteria

- [x] `TestSaveInLocksItsOwnFile` — a held lock on `s1.md` blocks a save to `s1.md` but not to `s2.md` in the same directory.
- [x] `TestMultiFileSaveTakesSortedLocks` — a story-and-sprint save driven from two goroutines in opposite argument order, 50 times each, does not deadlock within 30s.
- [x] `ListDir` returns names sorted, and an absent directory returns an empty slice with a nil error.
- [x] All twenty packages listed in the plan's Files make no direct filesystem call under `.procoder/`, and their exported signatures are unchanged.
- [x] `go test ./...` passes with no change to any existing test.
- [x] `procoder check` is clean.

## Evidence

- `internal/store/content.go` and `internal/store/content_test.go`, plus the
  migration of adr, analysis, answers, ask, backlog, bench, codeindex,
  config, docs, glossary, gitcmd, lessons, lint, plan, principles, review,
  spec, status, templates, todo and wizard. Committed as 56f80aa.
- Tests: TestSaveInLocksItsOwnFile (a lock on s1.md does not block s2.md),
  TestMultiFileSaveTakesSortedLocks, TestListDirAbsentIsEmpty,
  TestListDirIsSortedAndFilesOnly, TestInDirNameIsNotAPath, TestSaveDocLocks,
  TestRelRefusesOutsideRoot.
- Verified by the parity harness from task 7a: the goldens captured at
  c4bb353 still pass, which is what says the migration changed nothing.
  `go test ./...` green; `procoder check` clean, 39 files, 0 blocking.
- THREE HELPERS NOT IN THE PLAN, each because the migration ran into
  something the plan had not looked at:
  - `Rel` — spec.Files, plan.Files, analysis.Files, Item.Path and Task.Path
    all hand around ABSOLUTE .procoder/ paths, so their readers would have
    had to reach past the store to open what they were given. Rel refuses a
    path outside root, so "read what I was handed" cannot become "read
    anything".
  - `OpenIn` — the index tag file runs to megabytes and two readers scan it
    line by line; handing them the bytes would undo that.
  - `inDir` — the same separator refusal markerPath already had.
- TWO MORE HAND-ROLLED ATOMIC WRITES found and removed: internal/ask had one
  for the answers file and internal/codeindex another for its tag file.
  Both now take the lock neither had, so an index Refresh from the write
  hook can no longer race an `index build` in another session.
- internal/ask had parameters named `store`, shadowing the package; renamed
  to `decided` and `a`.
- The spec's Interfaces section was corrected during task 5 to show byte
  payloads and say why; that shape is what these twenty owners use.
