# store: atomic writes with temp sweeping

Status: open
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

- [ ] `TestAtomicWriteLeavesOriginalOnRenameFailure` — with the rename forced to fail, `WriteFile` errors and the target is byte-identical to its previous contents.
- [ ] `TestReaderNeverSeesPartialFile` — 2000 reads against a file being rewritten in a loop return only the complete old or complete new contents; replacing the body with `os.WriteFile` makes it fail.
- [ ] `TestTempFilesAreSwept` — a stale `.procoder-tmp-` file older than 30s in the destination directory is removed by the next successful write.
- [ ] `TestReadOnlyStateRefusesWrite` — with `.procoder/state` at mode 0555, the write is refused with the path in the message and no file under `.procoder/` changes. Skipped when running as root.
- [ ] `procoder check` is clean.

## Evidence

