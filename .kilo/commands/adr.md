---
description: "Architecture decision records: durable decisions with their date, context, and consequences — supersede, never rewrite."
---

The user invoked /procoder:adr with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

An ADR captures a cross-cutting decision at the moment it is made, so
the constraint that forced it survives the people and chats that knew
it. Records are immutable: a changed mind writes a NEW record and marks
the old one superseded.

- `procoder adr new <title>` — prints the next-numbered record;
  write it and fill Context (what forced this), Decision (what we chose
  and why over the alternatives), and Consequences (what gets easier
  and what gets harder). Then bring the status to accepted with the
  user's agreement — deciding is theirs.
- `procoder adr list` — every record; proposed first.
- `procoder adr check` — refuses hollow records, unknown statuses,
  and supersede references pointing nowhere. Fix what it names.

Record an ADR whenever a decision would surprise a newcomer or took
real debate: a dependency chosen, a boundary drawn, a ceiling accepted.
With arguments, run the matching subcommand; without, run `list` and
report.
