# On a fixture whose `.env.example` declares DATABASE_URL and REDIS_URL with only DATABASE_URL in `.env`, the output names REDIS_URL and no value from either file appears anywhere in the output — asserted by a test that plants a distinctive secret string as both values and greps the whole output for it.

Status: open
Created: 2026-08-19
Epic: sync-awareness
Sprint: -

## Description

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] On a fixture whose `.env.example` declares DATABASE_URL and REDIS_URL with only DATABASE_URL in `.env`, the output names REDIS_URL and no value from either file appears anywhere in the output — asserted by a test that plants a distinctive secret string as both values and greps the whole output for it.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
