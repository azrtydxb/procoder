# C-09: A spec question that has been ANSWERED in `answers.md` no longer blocks `spec check`: the spec reports COMPLETE with a note that the section still lists questions a human has decided. A question with no answer blocks exactly as it does today, and an answer whose question text has since changed does not count — verified by a test covering all three.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Make a human's answer count where the design is judged: an answered spec question no longer blocks `spec check`, while an unanswered one still does and an answer to different wording counts for nothing. The verdict moves; the silence does not, so nobody is shown a section full of questions and told it is finished.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-09: A spec question that has been ANSWERED in `answers.md` no longer blocks `spec check`: the spec reports COMPLETE with a note that the section still lists questions a human has decided. A question with no answer blocks exactly as it does today, and an answer whose question text has since changed does not count — verified by a test covering all three.

## Evidence

- TestAnAnsweredQuestionNoLongerBlocks walks all three states: unanswered blocks with 'unanswered' in the refusal, answered reports COMPLETE and names .procoder/ask/answers.md so nobody reads the section as finished, and a reworded question blocks again because the answer belonged to different wording.
- The reading is shared: spec.OpenQuestions is what both the collector offers and the checker judges, so the two can never disagree about what is still being asked.
- The bullet is stripped from the question text, so changing a dash to an asterisk does not silently discard an answer.
- This softens the rule merged in #94 exactly as far as D-5 says and no further: unanswered still blocks, a stale answer counts for nothing, and a spec passing on answers says so.
