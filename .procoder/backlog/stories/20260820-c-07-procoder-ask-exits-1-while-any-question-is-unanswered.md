# C-07: `procoder ask` exits 1 while any question is unanswered and 0 when all are, so a caller can tell the two apart.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Give a caller a verdict it can act on without parsing prose: 1 while any question is unanswered, 0 when none are, and 2 when the question could not be asked at all.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-07: `procoder ask` exits 1 while any question is unanswered and 0 when all are, so a caller can tell the two apart.

## Evidence

- Run exits 1 while any question is unanswered and 0 when none are, asserted in TestWithNoTerminalItWritesTheFileAndSaysSo (1) and TestAsecondRunWithNothingNewWritesNothing (0).
- Exit 2 is reserved for a question that could not be asked: TestAnUnreadableAnswersFileStopsEverything covers an unreadable record, where re-asking and writing over the top would destroy decisions already made.
- Live on this repository: `procoder ask < /dev/null` reported 10 questions and exited 1.
