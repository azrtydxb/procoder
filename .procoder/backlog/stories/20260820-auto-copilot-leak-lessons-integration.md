# auto-copilot-leak: lessons integration and `--from-copilot`

Status: done 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20
Sprint: 006-auto-copilot-leak-capture-copilots-auto-review-findings-as

## Description

Connect COPILOT-LEAKS.md to the existing lessons system. Entries in
COPILOT-LEAKS.md that have placeholder adaptations (`<...>`) are
UNLEARNED. Provide a way for `procoder lessons` to see them and for
the agent to convert COPILOT-LEAKS.md entries into formal LESSONS.md
entries.

## Acceptance criteria

- [x] The ledger has exactly one path constant, one writer and one reader — `internal/lessons` owns the file and `internal/copilot` writes through it.
- [x] `procoder copilot-leak --from-copilot` reports the ledger: learned, UNLEARNED, and exit 1 while any finding is unclassified.
- [x] The flag is documented in the usage text and accepted by the parser, pinned so the two cannot drift apart.
- [x] What Capture writes, the ledger report reads — pinned by a test across the two packages.
- [x] The merge flow references capturing leaks before finishing.

## Evidence

- copilot.Capture now calls lessons.RecordCopilotEntry; copilot's duplicate LedgerPath, entry(), appendLedger() and ledgerHeader are deleted (-99 lines in capture.go). internal/lessons' own comment already documented this as the intended design.
- TestWhatCaptureWritesTheLedgerReportReads: a captured finding round-trips to 'UNLEARNED … 1 finding(s), 1 unlearned', exit 1.
- TestCopilotLeakAcceptsEveryFlagTheUsageTextPromises reads the verdict from output, not the exit code (outside a GitHub repo the command legitimately exits 2). Proved by deleting the --from-copilot case: the test fails with 'usage promises --from-copilot but the parser refuses it'.
- Live: `procoder copilot-leak --from-copilot` exits 0 on this repo — the ledger is empty, which is the honest answer.
- commands/merge.md carries the copilot-leak step (commit d58fd2c).
