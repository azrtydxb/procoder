# adr

Status: complete

## Problem

Cross-cutting decisions — why SCIP over LSP, why one active sprint —
live only in changelogs and chat history. Specs capture one feature's
design; nothing captures the durable why with its date and context, so
six months later the constraint that forced a choice is gone and the
decision gets relitigated or accidentally reversed.

## Users

- Pascal and the agent recording a decision at the moment it is made.
- A future reader (human or agent) asking "why is it like this" before
  changing it.

## In scope

- `.procoder/adr/NNNN-<slug>.md` — architecture decision records,
  numbered, committed with the repo.
- `procoder adr new <title>` — prints the next-numbered ADR file
  (Status: proposed, Date, Context / Decision / Consequences sections
  with guidance comments) for the agent to write.
- `procoder adr list` — number, title, status, date; proposed first.
- `procoder adr check` — the controller: refuses (exit 1) on ADRs with
  empty Context, Decision, or Consequences; on statuses outside
  proposed/accepted/superseded-by-NNNN; and on superseded references
  pointing at ADRs that do not exist. Reports the count of records
  still `proposed` (informational — deciding takes a human).
- The audit sweep includes `adr check` findings when the directory
  exists.

## Out of scope

- Enforcing that decisions HAVE ADRs (no heuristic can know a decision
  happened).
- Approval workflows, sign-offs, or aging alarms on proposed ADRs.
- Rendering/site generation beyond the files being plain Markdown the
  docs domain already serves.

## Constraints

- Pure Go stdlib; package internal/adr.
- P-CONTROL: `new` prints; nothing ever rewrites an ADR (immutability
  is the point — supersede, never edit history).
- Honesty: unreadable ADR files are findings, not skips.
- File-name guard as everywhere (plain basenames only).

## Interfaces

- `procoder adr new <title> | list | check`.
- Usage text, docs.Commands, docs site, commands/adr.md skill +
  OpenCode twin.

## Data

- `.procoder/adr/NNNN-<slug>.md`: `# NNNN — <title>`,
  `Status: proposed|accepted|superseded-by-NNNN`, `Date: <YYYY-MM-DD>`,
  then Context, Decision, Consequences sections. Numbering is
  zero-padded four digits, next = max existing + 1.

## Edge cases

- Empty directory / no directory: list says how to start; check passes
  with "no records".
- Two files claiming the same number (merge collision): check flags
  the duplicate — numbering is identity.
- `superseded-by-0007` where 0007 is itself superseded: allowed
  (chains are legal); pointing at a missing file is the failure.
- A title slugifying to empty → exit 2.

## Failure modes

- Unreadable ADR → check refuses naming it.
- Directory unreadable → check fails loudly, never "no records".

## Acceptance criteria

- [ ] `adr new` prints 0001 for an empty repo and 0003 when 0002
      exists; the printed file carries all three sections and today's
      date; nothing is written by the binary.
- [ ] `adr check` refuses on an empty Decision section, an unknown
      status, a dangling supersede reference, and a duplicated number
      — each named — and passes on a valid set.
- [ ] `adr list` orders proposed before accepted and shows
      superseded-by targets.
- [ ] `procoder audit` on a repo with a broken ADR includes the
      finding.

## Open questions

<!-- none — decisions recorded above -->
