# The docs, checked against the binary rather than against memory

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 016-the-hooks-fed-real-payloads-and-the-docs-held-to-the-binary

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

- [x] Every command documented in `docs/commands.md` is invoked and its
      actual behaviour compared with the documented behaviour, with each
      disagreement recorded against the docs or the binary by name.
- [x] Every dispatch arm absent from `docs/commands.md` is listed as an
      undocumented command.
- [x] Documented exit codes are checked against observed exit codes for
      the clean, findings and usage-error cases.

## Evidence

- `scripts/e2e-docs-pass.sh`: **53 assertions, 0 failures** — 18 documented
  flags, 1 coverage check, 6 exit codes, 28 P-CONTROL digests. The count is
  checked against what the script can produce, after an earlier run
  returned 73 because the script was edited while bash was executing it.
- Every flag `docs/commands.md` advertises is accepted by the binary, and
  every flag `knownFlags` implements appears in the docs. Two did not:
  **`procoder docs --ack`**, which the gate's own blocking message tells an
  agent to run when a commit adds an exported symbol and touches no
  documentation, and **`principles --hook`**, documented in
  `portability.md` but absent from the command reference. Both recorded
  against the docs — the binary was right — and both now documented.
- Every dispatch arm is compared against the usage text and the docs by
  `TestEveryShippedCommandIsDiscoverable` in `internal/audit`, which found
  `config`, `review` and `analyze` shipping with no entry in `procoder
help`. That check runs in the suite rather than only in this campaign,
  because the drift it catches happens between campaigns.
- **Exit codes, against ADR 0003:** a clean read is 0, the gate over a
  clean file is 0 or 1 and never 2, an unformatted file is 1, and an
  unknown command, an unknown flag and a missing subcommand are each 2.
- **P-CONTROL:** 28 read-only invocations, each with the tree digested
  before and after, all byte-identical. This is the check that was hollow
  first: `procoder format` ran only over files that were already clean, so
  the branch that prints a rewritten file never executed, and a mutation
  making that branch write to disk passed untouched. With an unformatted
  file planted, the same mutation now fails with "`procoder format
greet/untidy.go` CHANGED the tree".
- Behavioural comparison beyond flags and exit codes comes from the clean
  pass (sprint 014, 53 invocations recorded) and the broken pass (sprint
  015, 21 defects caught by the command each entry claims owns it).
