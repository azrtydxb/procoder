# A quieter gate says that it is quieter

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

A gate that checks less and does not say so is indistinguishable from
one that checked everything and found nothing. That is the failure this
project exists to prevent, and it does not stop being that failure because
the reduction was deliberate.

Every run names the mode it ran in. A non-adopting run also says how to
change it, so somebody who wanted the full gate is not left guessing why it
went quiet.

## Acceptance criteria

- [x] The gate's summary line names the mode it ran in.
- [x] A non-adopting run says how to get the full gate.

## Evidence

`TestTheGateAnnouncesItsScope` — the mode line, the "NOT checked here"
warning, and the summary. The summary needed its own branch: "0 clean, 0
unformatted" over files nothing looked at reads as a formatting pass, so
the universal run says `N file(s) not formatting-checked` instead. That
assertion was added only after the mutation survived without it.
