# The clean pass — every offline command against healthy code

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: -

## Description

The first question a new adopter asks procoder is "is my repository
okay", and the worst possible answer is a finding they cannot act on
because the code was already correct. This pass asks that question 78
times, once per command arm in the dispatch, against a fixture built to
be clean.

A finding here is a procoder defect and is recorded with the command
that raised it. So is a crash, a panic, a usage error on documented
flags, and a command that produces no output at all.

The pass records three verdicts, never two. A command whose tool is not
installed on this machine is NOT RUN with the tool named — not a pass,
and not a procoder defect either, because the gap is the machine. The
campaign refusing to distinguish those would be the campaign committing
the failure it is hunting.

## Acceptance criteria

- [ ] Every offline command is invoked against the healthy fixture and
      its verdict recorded; any finding raised against correct code is
      reported as a procoder defect with the command that raised it.
- [ ] The recorded verdicts are PASS, FINDING or NOT RUN, and every NOT
      RUN names why it could not run.
- [ ] The count of commands invoked is reconciled against the dispatch
      arms in `cmd/procoder/main.go`; any arm not invoked is listed with
      the reason.

## Evidence
