---
description: "The project layer: milestones, epics, and user stories with refusing controllers — spec-seeded, sprint-ready."
---

The user invoked /procoder:backlog with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

The backlog is where a spec-first project lives: milestones group epics,
epics group user stories, and the story is the execution unit with the
same rigor as a todo task. The todo list itself stays separate — it is
for standalone work not born from a spec.

- `procoder backlog seed <spec> [--milestone <id>]` — decompose a
  COMPLETE spec into an epic plus one story per acceptance criterion.
  Everything is PRINTED for you to review and write; the epic records
  the spec and a fingerprint of its acceptance criteria, so the board
  can flag drift when the contract changes later.
- `procoder backlog milestone <title>` /
  `procoder backlog epic <title> [--milestone <id>]` /
  `procoder backlog story <title> --epic <id>` — print the file for
  each level; write it, then fill the real content (a story needs a
  real description and testable acceptance criteria before work starts).
- `procoder backlog bug <title> [--epic <id>] [--severity sN]` —
  a defect is a story with a severity and a pre-seeded regression-test
  criterion; close refuses without the severity. Triage honestly:
  s1/s2 jump the queue, and the regression test is not optional.
- `procoder backlog board` — the tree: milestones → epics → stories
  with statuses, sprint tags, spec-drift flags, and orphans. Run this
  to orient before pulling work.
- `procoder backlog list` — the flat listing, open items first.
- `procoder backlog close story <id>` — the quality controller:
  it REFUSES until the description is real, every acceptance criterion
  is checked, evidence is recorded, and the gate is clean. Fix what it
  names and rerun; never argue with a refusal by weakening criteria.
- `procoder backlog close epic <id>` — refuses while any of the
  epic's stories is open; warns on spec drift.
- `procoder backlog close milestone <id>` — refuses while any of its
  epics is open.

Sprints pull stories from the backlog — see /procoder:sprint. With
arguments, run the matching subcommand and act on its output. Without
arguments, run `board` and report the project's state.
