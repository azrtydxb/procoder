# A malformed payload allows the command and says the gate could not judge — verified by a test feeding broken stdin.

Status: done 2026-08-19
Created: 2026-08-19
Epic: gate-enforcement
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

A broken hook never wedges a session.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A malformed payload allows the command and says the gate could not judge — verified by a test feeding broken stdin.

## Evidence

- TestMalformedPayloadAllowsAndSaysItCouldNotJudge green. Judgment recorded: unparseable stdin allows (there may be no commit at all), while a gate that RAN and timed out denies under block — a real inability to verify a real commit.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
