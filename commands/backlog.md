---
description: "The project layer: milestones, epics, and user stories with refusing controllers — spec-seeded, sprint-ready."
---

The user invoked /procoder:backlog with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

The backlog is where a spec-first project lives: milestones group epics,
epics group user stories, and the story is the execution unit with the
same rigor as a todo task. The todo list itself stays separate — it is
for standalone work not born from a spec.

- `launcher.sh backlog seed <spec> [--milestone <id>]` — decompose a
  COMPLETE spec into an epic plus one story per acceptance criterion.
  Everything is PRINTED for you to review and write; the epic records
  the spec and a fingerprint, so the board can flag drift later.
- `launcher.sh backlog milestone <title>` /
  `launcher.sh backlog epic <title> [--milestone <id>]` /
  `launcher.sh backlog story <title> --epic <id>` — print the file for
  each level; write it, then fill the real content (a story needs a
  real description and testable acceptance criteria before work starts).
- `launcher.sh backlog board` — the tree: milestones → epics → stories
  with statuses, sprint tags, spec-drift flags, and orphans. Run this
  to orient before pulling work.
- `launcher.sh backlog list` — the flat listing, open items first.
- `launcher.sh backlog close story <id>` — the quality controller:
  it REFUSES until the description is real, every acceptance criterion
  is checked, evidence is recorded, and the gate is clean. Fix what it
  names and rerun; never argue with a refusal by weakening criteria.
- `launcher.sh backlog close epic <id>` — refuses while any of the
  epic's stories is open; warns on spec drift.
- `launcher.sh backlog close milestone <id>` — refuses while any of its
  epics is open.

Sprints pull stories from the backlog — see /procoder:sprint. With
arguments, run the matching subcommand and act on its output. Without
arguments, run `board` and report the project's state.
