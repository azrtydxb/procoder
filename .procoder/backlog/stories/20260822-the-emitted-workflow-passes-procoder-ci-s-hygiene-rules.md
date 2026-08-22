# The emitted workflow passes `procoder ci`'s hygiene rules: asserted by feeding the emitter's own output back into `ciops.Check` and requiring no findings.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

One binary contains both the generator and the checker. If what it
emits fails what it checks — unpinned actions, a job with no timeout,
concurrency that does not cancel — then one of the two is wrong, and a
user meets that contradiction on their first run.

Done means the emitter's own output is fed back into `ciops.Check` and
produces no findings.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] The emitted workflow passes `procoder ci`'s hygiene rules: asserted by feeding the emitter's own output back into `ciops.Check` and requiring no findings.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
