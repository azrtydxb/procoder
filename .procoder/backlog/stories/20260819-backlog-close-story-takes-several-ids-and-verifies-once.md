# backlog close story takes several ids and verifies once

Status: done 2026-08-19
Created: 2026-08-19
Epic: batch-verification
Sprint: 005-batch-close-verify-once-not-per-story-0320

## Description

Closing a sprint's worth of stories one at a time re-runs the gate and
the whole suite once per story: 27 closes took roughly thirteen minutes
of repeated identical verification, and from outside it looked like a
hang. Each close is correct; the repetition is the waste. `backlog
close story <id>...` should accept several ids, verify once, and then
apply that one verdict to every story whose own criteria and evidence
pass — refusing the individual stories that do not, by name.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `backlog close story a b c` runs the gate and the suite exactly
      once, verified by a test counting invocations.
- [x] A story in the batch whose criteria or evidence are incomplete is
      refused by name while the others still close.
- [x] The single-id form behaves exactly as today.

## Evidence

- TestCloseStoriesVerifiesOnceForTheWholeBatch: three stories close with
  gate=1 and suite=1 invocations, asserted by counters, and all three
  files carry `Status: done` afterwards.
- TestCloseStoriesRefusesTheIncompleteOneByName: the hollow story is
  refused by name and stays open while the complete one still closes;
  the batch exits 1.
- TestCloseStoriesOfOneMatchesTheSingleForm: identical exit code and
  identical output to CloseStoryWith, so nothing about the single-id
  path changed.
- Live on a fixture: three serial closes took 2s, the same three batched
  took 0s. On this repository one verification (check + test) measures
  27s, so a 27-story sprint goes from about 729s of identical
  re-verification to 27s.
- The CLI takes `close story <id>...`; `close epic` and `close milestone`
  still require exactly one id.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
