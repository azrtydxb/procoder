# The emitted workflow runs `procoder docs --external` and not the bare `procoder docs`: the offline half already rides the gate, so emitting it again in CI would repeat the commit's answer while leaving link rot — the only part CI can see — unchecked.

Status: open
Created: 2026-08-22
Epic: ci-that-procoder-writes
Sprint: -

## Description

`procoder docs` and `procoder docs --external` are different checks.
The offline half already rides the commit gate, so emitting it in CI
spends a job repeating the answer the commit gave, while leaving link
rot unchecked — and link rot is the one thing only CI can see, because
it happens when somebody else's server changes and no commit here
touches anything.

Done means the emitted step is the external one, asserted so an
implementation cannot satisfy the neighbouring criterion either way.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] The emitted workflow runs `procoder docs --external` and not the bare `procoder docs`: the offline half already rides the gate, so emitting it again in CI would repeat the commit's answer while leaving link rot — the only part CI can see — unchecked.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
