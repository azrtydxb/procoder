# The broken pass — one planted defect per class, each must be caught

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: -

## Description

This is the story the epic exists for. A gate that finds nothing on
healthy code and also nothing on a planted defect is not clean, it is
silent, and silence is exactly the shape of every bug found in the
session that prompted this work.

The fixture is seeded with one deliberate defect per class procoder
claims to catch: unformatted source in each language, a lint finding, a
hardcoded secret, a SAST finding, a manifest pinning a known-vulnerable
version, a conflict marker, an oversized file, AI attribution in a
commit message, a debt marker with no revisit trigger, agent rules
drifted from the principles, and a doc reference pointing at a file that
does not exist.

Each must be caught, and caught by the command that owns it, and named
in the output specifically enough that somebody could fix it without
already knowing what was planted. A defect caught by the wrong command,
or reported so vaguely the reader cannot locate it, is recorded as a
finding too.

## Acceptance criteria

- [ ] One deliberate defect per class procoder claims to catch is
      planted, and each is caught and named by the command that owns it;
      any that is not caught is reported with the command that missed it.
- [ ] The output for each caught defect names the file, and where the
      class has a location, the line — enough to act on without knowing
      what was planted.
- [ ] The unformatted-source defect is planted in every one of the
      twelve languages separately, since a formatter table has twelve
      independent rows.

## Evidence
