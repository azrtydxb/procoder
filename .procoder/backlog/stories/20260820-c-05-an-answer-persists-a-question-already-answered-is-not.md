# C-05: An answer persists: a question already answered is not asked again on the next run, and the same question with its text changed IS asked again — verified by a test that answers, re-runs, edits the question, and re-runs.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Make an answer outlive the session that gave it. Answers are keyed to a fingerprint of the question, so an unchanged question is never asked twice and a reworded one is asked again — the old answer belonged to a different question.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-05: An answer persists: a question already answered is not asked again on the next run, and the same question with its text changed IS asked again — verified by a test that answers, re-runs, edits the question, and re-runs.

## Evidence

- TestAnAnswerSurvivesUntilTheQuestionChanges answers a question, re-runs and finds it not asked again, then rewords the question and finds it asked again.
- Keyed by answers.Key(source, origin, text) — a fingerprint of what was ASKED, so an answer belongs to a question rather than to a position in a list.
- DEVIATION from the plan, recorded: the plan's risk table said 'clear answers on each run; the file is ephemeral per session'. That contradicted D-2 and would have re-asked everything at every session start; the plan was corrected before implementation rather than followed.
