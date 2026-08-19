# backlog close story takes several ids and verifies once

Status: open
Created: 2026-08-19
Epic: batch-verification
Sprint: -

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

- [ ] `backlog close story a b c` runs the gate and the suite exactly
      once, verified by a test counting invocations.
- [ ] A story in the batch whose criteria or evidence are incomplete is
      refused by name while the others still close.
- [ ] The single-id form behaves exactly as today.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
