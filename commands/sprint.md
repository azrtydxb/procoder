---
description: "Scope-boxed sprints over the backlog: one active sprint, explicit carry-over, a close that refuses to hide unfinished work."
---

The user invoked /procoder:sprint with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

A sprint is a commitment, not a calendar: a goal plus the stories pulled
into it, opened and closed explicitly. One sprint is active at a time —
that is the WIP limit, and the point.

- `launcher.sh sprint open <goal>` — refuses while a sprint is active;
  otherwise prints the sprint file for you to write. State a real goal:
  what this sprint delivers, not a date range.
- `launcher.sh sprint pull <story-id>...` — commit stories to the
  active sprint. Done, missing, or already-committed stories are
  refused individually; the rest still pull.
- `launcher.sh sprint status` — goal, committed stories with statuses,
  done/total and carried counts. Run it before claiming progress.
- `launcher.sh sprint carry <story-id> <reason>` — return an unfinished
  story to the backlog, with the reason recorded in the story file.
  Lean makes unfinished work visible; carrying without a reason is
  refused.
- `launcher.sh sprint close` — REFUSES while a committed story is
  neither done nor carried. Finish it (`backlog close story <id>`) or
  carry it with a reason, then close; the sprint file gets a Result
  section with committed/done/carried counts and a Retro scaffold.
- Fill the Retro before moving on — what slowed us, what we change,
  one adaptation worth keeping. The retro is the price of the next
  sprint: `sprint open` refuses while the last one's Retro is empty
  (`[sprint] retro = "off"` opts a repo out).

With arguments, run the matching subcommand and act on its output.
Without arguments, run `status` and report where the sprint stands.
