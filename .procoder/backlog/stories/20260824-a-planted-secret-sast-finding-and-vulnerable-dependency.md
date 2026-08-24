# Security, proved in both directions

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: -

## Description

The security domain is the one where a silent green costs the most, and
it is also the one with the most moving parts: a secret scanner, a SAST
pass, a dependency audit, each with configurable severity and each
capable of reporting clean because it never ran.

Three defects are planted — a credential, a SAST finding, a manifest
pinning a known-vulnerable version — and each must block at the severity
the docs claim. That proves the check fires.

Then each is relaxed through the documented configuration and must stop
blocking. That proves the configuration is real rather than decorative,
and it is the half normally skipped: a knob that quietly does nothing
looks identical to a knob that works, right up until somebody depends on
it.

A flagged secret's value never appears in output, questions, `QA.md` or
a hook payload, and this story asserts that too.

## Acceptance criteria

- [ ] A planted secret, SAST finding and vulnerable dependency each block
      at the documented severity, and each stops blocking when the
      documented configuration relaxes it.
- [ ] The flagged secret's value appears in no output, no question, no
      `QA.md` entry and no hook payload — asserted by searching for it.
- [ ] A security check that could not run — scanner absent, manifest
      unreadable — reports NOT RUN naming the reason, and never reads as
      clean.

## Evidence
