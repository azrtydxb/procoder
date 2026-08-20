---
description: "Turn an approved spec into an implementation plan an engineer with zero context could execute — with a quality controller that blocks placeholders and hollow tasks."
---

The user invoked /procoder:plan with arguments:

The command below is the `procoder` binary on PATH.

Plans live under `.procoder/plans/`, the middle link of the chain:
spec (what and why) → plan (how, exactly) → todo (tracked, quality-gated
execution). Write the plan for an engineer who is skilled but knows
nothing about this codebase and its domain: every task self-contained,
every value literal, nothing left to taste.

With a name in the arguments, start (or resume) that plan. With `check`
or `list`, run the matching subcommand. With no arguments, run
`procoder plan list` and report.

The workflow:

1. Start from a COMPLETE spec (`procoder spec check <name>` says so).
   No spec → run /procoder:spec first; the thinking happens before the
   plan, not inside it.
2. `procoder plan template <name>` prints the shape; write it to the
   printed path and fill it:
   - **Goal** — one sentence. **Architecture** — two or three.
   - **Constraints** — project-wide requirements every task inherits
     (version floors, naming rules, exact copy), taken verbatim from the
     spec. Every task implicitly includes this section.
   - **Tasks** (`## Task N: <name>`) — the smallest unit that carries its
     own test cycle and is worth a reviewer's gate. Fold setup, config,
     and docs into the task whose deliverable needs them; split only
     where a reviewer could reject one task while approving its
     neighbour. Each task carries:
     - `Files:` — every file created or modified, one responsibility each.
     - `Interfaces:` — the exact names and signatures this task consumes
       from earlier tasks or produces for later ones. A task's
       implementer sees only their own task; this line is how they learn
       what their neighbours call things.
     - Checkbox steps, one action each: write the failing test (with the
       literal test code or command and `expect FAIL with "..."`),
       implement minimally, run to pass, commit.
3. Never write these — they are plan failures, not plans: "TBD", "TODO",
   "implement later", "add appropriate error handling", "handle edge
   cases", "write tests for the above" (without the test code), or
   "similar to Task N" (repeat the code — tasks are read out of order).
4. Run `procoder plan check <name>` after each pass; it names the
   remaining gaps. Keep writing until it says COMPLETE. Then self-review
   once against the spec: point to the task that covers each spec
   requirement, and check names stay consistent across tasks (a function
   renamed between Task 3 and Task 7 is a bug).
5. When COMPLETE: seed the todo list — `procoder todo add` one task
   each, acceptance criteria from the task's steps (see /procoder:todo) —
   and execute task by task, gate-clean after each. If reality
   contradicts the plan mid-build, update the plan first and re-check.
