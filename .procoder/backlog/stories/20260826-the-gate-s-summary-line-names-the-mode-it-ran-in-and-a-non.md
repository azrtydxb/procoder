# A quieter gate says that it is quieter

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

A gate that checks less and does not say so is indistinguishable from
one that checked everything and found nothing. That is the failure this
project exists to prevent, and it does not stop being that failure because
the reduction was deliberate.

Every run names the mode it ran in. A non-adopting run also says how to
change it, so somebody who wanted the full gate is not left guessing why it
went quiet.

## Acceptance criteria

- [ ] The gate's summary line names the mode it ran in.
- [ ] A non-adopting run says how to get the full gate.

## Evidence
