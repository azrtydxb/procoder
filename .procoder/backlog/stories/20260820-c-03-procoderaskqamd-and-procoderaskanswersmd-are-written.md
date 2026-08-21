# C-03: `.procoder/ask/QA.md` and `.procoder/ask/answers.md` are written at those paths, and a second run with no new questions rewrites neither.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Put the two files where everything else can find them, and leave them alone when there is nothing new to say. A tool that rewrites its own state on every read produces a diff nobody can trust.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-03: `.procoder/ask/QA.md` and `.procoder/ask/answers.md` are written at those paths, and a second run with no new questions rewrites neither.

## Evidence

- Both files are written under .procoder/ask/, verified by TestWithNoTerminalItWritesTheFileAndSaysSo reading QA.md back and finding the question text in it.
- TestAsecondRunWithNothingNewWritesNothing pins the second half: with everything answered, the run exits 0, the answers file's mtime is unchanged, and no questions file is written at all. A tool that rewrites its own state on every read makes a diff nobody can trust.
