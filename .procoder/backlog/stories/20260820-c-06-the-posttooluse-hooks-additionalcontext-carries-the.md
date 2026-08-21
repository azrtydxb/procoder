# C-06: The PostToolUse hook's `additionalContext` carries the pending questions and the instruction not to guess, and carries nothing when none are pending.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Carry the pending questions into the place the coder actually reads, with the instruction that matters more than the list: stop, relay, do not answer. Say nothing at all when nothing is pending, because a hook that speaks with nothing to say trains the reader to skip it.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-06: The PostToolUse hook's `additionalContext` carries the pending questions and the instruction not to guess, and carries nothing when none are pending.

## Evidence

- internal/hook.askPart injects a `== q&a` section into the PostToolUse additionalContext, carrying up to five pending questions and the instruction not to guess.
- TestTheHookCarriesPendingQuestionsAndTheInstruction asserts the section carries the marker, the question, 'Do NOT guess' and the --file route — and that nothing at all is emitted when no question is pending, because a hook that speaks with nothing to say trains the reader to skip it.
