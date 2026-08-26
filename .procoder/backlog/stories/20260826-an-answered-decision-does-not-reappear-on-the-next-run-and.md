# An answered decision does not reappear on the next run, and the same decision with edited text does.

Status: done
Created: 2026-08-26
Epic: decisions-reach-the-user
Sprint: 022-a-decision-the-agent-cannot-make-reaches-the-user

## Description

A decision answered once must not be asked again, and a decision whose
wording changed must be — the old answer belonged to a different
question.

Done when this behaviour comes from the machinery `answers` already
provides rather than from anything new invented here.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] An answered decision does not reappear on the next run, and the same decision with edited text does.

## Evidence

`TestAnEditedDecisionIsANewQuestion` pins that the key hashes the text,
so the behaviour falls out of `answers.Key` rather than being
re-implemented. Killed by making `Question.Key()` ignore `Text`.

Walked the full loop against a scratch repository: decision asked →
answered via `procoder ask --file` → `all 1 question(s) already answered`
→ heading reworded → asked again.
