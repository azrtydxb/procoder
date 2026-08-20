# auto-copilot-leak: issue capture and COPILOT-LEAKS.md

Status: done 2026-08-20
Epic: auto-copilot-leak
Created: 2026-08-20
Sprint: 006-auto-copilot-leak-capture-copilots-auto-review-findings-as

## Description

Complete the capture path: when the user says yes, create GitHub issues
for each sanitised finding and append entries to a new COPILOT-LEAKS.md
scratch ledger. No integration with lessons yet — that comes in a later
story.

## Acceptance criteria

- [x] Capture opens one GitHub issue per sanitised finding and records each in `.procoder/github/COPILOT-LEAKS.md` as UNLEARNED.
- [x] The two halves are independent: a failed issue still writes the ledger, an unwritable ledger still creates the issues, and every failure is reported rather than swallowed.
- [x] A finding that sanitises to nothing is skipped with a note instead of publishing an empty issue.
- [x] The ledger is created with a self-explaining header on first write — deliberately with no example entry.
- [x] An issue template for Copilot findings ships under `.github/ISSUE_TEMPLATE/` and is mirrored by `procoder templates`.

## Evidence

- `go test ./internal/copilot/` green — TestCaptureOpensAnIssueAndRecordsAnUnlearnedEntry, TestIssueFailureStillWritesTheLedger, TestUnwritableLedgerStillCreatesTheIssues, TestEmptyBodyIsSkippedWithANote.
- DEVIATION: the template shipped as `.github/ISSUE_TEMPLATE/copilot-leak.yml`, a GitHub issue form, not the specced `.md` body template — `createIssue` builds its own body, so the file's job is human filing.
- DEVIATION: no seeded ledger file with an example entry. lessons.Parse reads every `## ` heading as an entry, so a committed example would make an empty ledger report a phantom unlearned lesson; the header is written on first capture instead.
- `gh` is never invoked in tests — a stub on PATH records the argv, asserted against.
