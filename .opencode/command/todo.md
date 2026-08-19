---
description: "The quality-gated task list: tasks with real descriptions, testable acceptance criteria, and evidence — a task only closes when the controller agrees it is done."
---

The user invoked /procoder:todo with arguments:

The command below is the `procoder` binary on PATH.

Tasks live as Markdown files under `.procoder/todo/`, one file per task.
This replaces throwaway checklists: every task carries a description that
says what "done" looks like, acceptance criteria a reviewer could verify,
and evidence of what proved them. `procoder todo close` is the quality
controller — it REFUSES to close a task until every criterion is checked,
the evidence is recorded, and the gate is clean. You do the work and the
verification; the binary judges.

Subcommands (run the one matching the arguments; with no arguments, run
`list` and report the open tasks):

- `procoder todo add <title>` — prints the task file and its path. Write
  the file, then REPLACE the placeholders: a real description (why the task
  exists and what done looks like), and one `- [ ]` line per testable
  acceptance criterion. Do this before starting the work — criteria written
  after the fact just describe whatever happened.
- `procoder todo list` — every task, open first.
- `procoder todo show <id>` — the full task file.
- `procoder todo close <id>` — the quality controller. Before running
  it: verify each criterion yourself (run the test, exercise the behaviour),
  check its box only when it is true, and fill `## Evidence` with what you
  ran and what the output proved — one line per criterion. Then close. If
  it refuses, it names exactly what is missing: do that work, don't game
  the checkboxes. A checked box you did not verify is a lie the controller
  cannot catch — the evidence section exists so the user can.

  What counts as evidence — if you haven't run the verification command
  fresh, you cannot check the box:
  - "tests pass" = test output with 0 failures, from THIS state of the
    code — not an earlier run, not "should pass".
  - "bug fixed" = the test for the original symptom passes — and for a
    regression test, the red-green proof: revert the fix, watch the test
    FAIL, restore, watch it pass. A regression test never seen red proves
    nothing.
  - "build/lint clean" = the command's exit 0 and its output read, not
    extrapolated from a partial check.
  - a subagent's "done" = verified in the diff and by running its tests
    yourself — an agent's success report is a claim, not evidence.
  - "should", "probably", "seems to" in your evidence line means the box
    stays unchecked.

Rules:

- Never edit `Status:` by hand — only `todo close` moves a task to closed.
- Track every piece of multi-step work here, not in throwaway TodoWrite
  lists: these files survive the session and the user can read them.
- When a spec exists (`procoder spec list`), seed tasks from its
  acceptance criteria — one task per coherent group, criteria copied in.
