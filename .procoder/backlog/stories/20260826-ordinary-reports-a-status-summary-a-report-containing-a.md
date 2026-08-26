# Ordinary reports — a status summary, a report containing a rhetorical question, a completed-work message — all exit 0, per `TestOrdinaryReportsDoNotBlock`; fails if the detector fires on a question mark.

Status: done
Created: 2026-08-26
Epic: an-unasked-decision-does-not-end-the-turn
Sprint: 025-an-unasked-decision-does-not-end-the-turn

## Description

The criterion that matters most. A false block fires on correct work at the end of every single turn — the #172 and #185 failure at maximum frequency, and it would discredit every other check procoder makes.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Ordinary reports — a status summary, a report containing a rhetorical question, a completed-work message — all exit 0, per `TestOrdinaryReportsDoNotBlock`; fails if the detector fires on a question mark.

## Evidence

`TestOrdinaryReportsDoNotBlock`, whose fixtures are real messages from the session that built this: a status summary, a rhetorical question, a finding report, completed work, and an ask-shaped phrase mid-narration.

One genuine false positive was caught this way — "I asked whether you want me to keep it, and you said yes" is narration about a decision already taken, in the same words as an ask. An interrogative phrase now counts only with a question mark in the same sentence, while a deferring phrase ("say the word", "your call") counts on its own. `TestNarrationAboutAPastDecisionIsNotAnAsk` and `TestADecisionHandedOverWithoutAQuestionMarkStillBlocks` pin both sides.

Killed by matching on a bare question mark, and separately by dropping the question-mark test.
