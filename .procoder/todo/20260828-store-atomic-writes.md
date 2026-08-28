# store: atomic writes with temp sweeping

Status: closed 2026-08-28
Created: 2026-08-28

## Description

Plan task 2 of .procoder/plans/service-state-seam.md.

Every .procoder/ write today is a plain os.WriteFile, which truncates
first. A process killed mid-write leaves a truncated claims.json, and the
next reader reports a corrupt ledger for a crash that had nothing to do
with it.

Done means WriteFile writes a temp file in the destination directory
(os.Rename is only atomic within a filesystem), fsyncs it, and renames it
over the target — so a reader sees the whole old file or the whole new
one, and a failure leaves the target untouched.

## Acceptance criteria

- [x] `TestAtomicWriteLeavesOriginalOnRenameFailure` — with the rename forced to fail, `WriteFile` errors and the target is byte-identical to its previous contents.
- [x] `TestReaderNeverSeesPartialFile` — 2000 reads against a file being rewritten in a loop return only the complete old or complete new contents; replacing the body with `os.WriteFile` makes it fail.
- [x] `TestTempFilesAreSwept` — a stale `.procoder-tmp-` file older than 30s in the destination directory is removed by the next successful write.
- [x] `TestReadOnlyStateRefusesWrite` — with `.procoder/state` at mode 0555, the write is refused with the path in the message and no file under `.procoder/` changes. Skipped when running as root.
- [x] `procoder check` is clean.

## Evidence

- `internal/store/atomic.go` and `internal/store/atomic_test.go`,
  committed as 344a81f. `go test ./internal/store/` passes.
- Four tests: TestAtomicWriteLeavesOriginalOnRenameFailure,
  TestReaderNeverSeesPartialFile (2000 reads against a file being
  rewritten in a loop), TestTempFilesAreSwept,
  TestReadOnlyStateRefusesWrite (skipped as root, where the mode does not
  apply).
- Mutation-checked, snapshot taken and restored around each: inserting
  `os.WriteFile(target, nil, perm)` at the top of WriteFile — the
  truncate-first behaviour every write had before this change — fails both
  TestAtomicWriteLeavesOriginalOnRenameFailure and
  TestReaderNeverSeesPartialFile; replacing `sweep(dir)` with `_ = dir`
  fails TestTempFilesAreSwept with `stale temp file survived a successful
write`.
- The temp file is created in the destination directory, not os.TempDir:
  os.Rename is only atomic within a filesystem.
- KNOWN GAP: no test covers the `f.Sync()`. Its failure mode is power loss
  between the rename and the data reaching disk, and no harness here can
  produce that. The call stays because the pattern is only correct with
  it, but this criterion set does not prove it.
- `procoder check` clean; the commit gate passed.
