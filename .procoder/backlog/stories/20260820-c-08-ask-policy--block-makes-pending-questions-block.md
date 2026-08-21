# C-08: `[ask] policy = "block"` makes pending questions block `procoder check`; the default `report` lists them and leaves the gate's verdict unchanged — both verified by test.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Let a repository decide how hard the rule is. Report by default — a question is a request for judgement, not a defect, and failing a commit on one stops work the person who could unblock it may not be awake for — with block available for a repository that wants the harder rule.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-08: `[ask] policy = "block"` makes pending questions block `procoder check`; the default `report` lists them and leaves the gate's verdict unchanged — both verified by test.

## Evidence

- TestTheAskPolicyDecidesWhetherQuestionsBlock asserts report (the default) produces non-blocking findings, `[ask] policy = "block"` makes the same findings blocking, and an answered question produces none at all.
- TestAnUnreadableRecordIsNotAnEmptyGateVerdict pins the honesty rule: a collection that could not run reports nothing rather than 'no questions', and the command says out loud what it could not read.
- Live: `procoder check` on this repository reported 11 (ask) lines, none blocking.
