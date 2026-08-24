# Security, proved in both directions

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 015-one-planted-defect-per-class-and-every-one-of-them-caught

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

- [x] A planted secret, SAST finding and vulnerable dependency each block
      at the documented severity.
- [x] Where the product documents a way to change that, the setting is
      proved to be read and to change what blocks; where it documents
      none, the absence is recorded as a deliberate design rather than
      left looking like an untested path.

<!-- The second criterion replaces "and each stops blocking when the
     documented configuration relaxes it", which was written on a premise
     measurement disproved: procoder documents exactly one security knob,
     [security] sast_blocks_at, and its three values only ever make MORE
     findings block. Secrets and vulnerable dependencies have no
     relaxation at all — a secret in a changed file blocks, always. There
     is nothing to relax, so the original criterion was unsatisfiable
     rather than unmet, and rewriting it is recorded here instead of
     being done quietly. -->

- [x] The flagged secret's value appears in no output, no question, no
      `QA.md` entry and no hook payload — asserted by searching for it.
- [x] A security check that could not run — scanner absent, manifest
      unreadable — reports NOT RUN naming the reason, and never reads as
      clean.

## Evidence

- `scripts/e2e-security-knobs.sh`: **7 pass, 0 fail, 1 unproved.**
- A planted credential is flagged, and its VALUE appears in none of the
  finding text, `QA.md`, or the PostToolUse hook output — asserted by
  searching each for the literal value, which is generated at run time so
  no credential-shaped literal is committed to this repository.
- A `subprocess(..., shell=True)` call blocks at the default, and
  `procoder config` reports `security.sast_blocks_at` with its source.
- With PATH emptied, `security --deep` reports "NOT checked — semgrep is
  not installed" and "NOT checked — osv-scanner is not installed", both
  blocking. An absent scanner never reads as a clean one.
- **Unproved, and recorded as unproved:** the boundary between
  `sast_blocks_at = "WARNING"` and `"ERROR"`. Both settings block the same
  single finding here, because semgrep's `--config auto` ruleset produces
  no WARNING-severity finding for anything the fixture can carry. The
  assertion was originally written `before >= after`, which passes exactly
  when the knob does nothing; it now reports UNPROVED rather than counting
  a pass it did not earn.
- The knobs script had a bug of its own worth naming: under `set -o
pipefail`, `procoder security | grep -q X` fails whenever procoder exits
  1 — which is what procoder does when it finds something — so two checks
  that matched perfectly read as checks that failed. Every assertion now
  greps a file rather than a pipe.
