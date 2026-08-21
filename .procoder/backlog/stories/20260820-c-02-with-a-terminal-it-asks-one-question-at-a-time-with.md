# C-02: With a terminal it asks one question at a time; with `/dev/null` or a pipe on stdin it asks nothing, writes `.procoder/ask/QA.md`, names the file and the `--file` route, and does not hang.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Ask a person when there is a person to ask, and write the questions down when there is not. One question at a time on a terminal, with an empty answer treated as a skip rather than a decision; with no terminal, no prompt at all — a pipe cannot answer, and asking it would hang or record silence as consent.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-02: With a terminal it asks one question at a time; with `/dev/null` or a pipe on stdin it asks nothing, writes `.procoder/ask/QA.md`, names the file and the `--file` route, and does not hang.

## Evidence

- TestWithNoTerminalItWritesTheFileAndSaysSo runs against /dev/null and asserts the run does not hang, writes QA.md, and names both the file and the --file route.
- The terminal test uses copilot.CanAsk, which already knows /dev/null is a character device and therefore nobody to ask — the fix made earlier this session, reused rather than rediscovered.
- With a terminal, askEach puts one question at a time and treats an empty answer as a skip: a blank line is somebody deferring, and recording it would silence the question forever.
