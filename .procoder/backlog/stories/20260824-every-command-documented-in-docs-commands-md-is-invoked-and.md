# The docs, checked against the binary rather than against memory

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: -

## Description

`docs/commands.md` is written by hand and edited every time a command
changes, which means it is correct exactly as often as somebody
remembered. It is also the first thing an adopter reads, so a sentence
that no longer matches the binary is a defect with a wide blast radius.

Every documented command is invoked and its actual behaviour compared
with the documented behaviour: the flags it accepts, the exit codes it
returns, whether it writes anything, and what it prints. Disagreements
are recorded against one side or the other by name — because fixing the
binary to match a sentence somebody wrote is how a doc error becomes a
behaviour regression.

The reverse direction counts too: a command in the dispatch that
`docs/commands.md` never mentions is undocumented, and that is a finding
against the docs.

## Acceptance criteria

- [ ] Every command documented in `docs/commands.md` is invoked and its
      actual behaviour compared with the documented behaviour, with each
      disagreement recorded against the docs or the binary by name.
- [ ] Every dispatch arm absent from `docs/commands.md` is listed as an
      undocumented command.
- [ ] Documented exit codes are checked against observed exit codes for
      the clean, findings and usage-error cases.

## Evidence
