---
description: "Spec-first design: a gap-closing interview that produces a complete spec, with a quality controller that blocks until every section is answered and every question resolved."
---

The user invoked /procoder:spec with arguments:

The command below is the `procoder` binary on PATH.

Specs live under `.procoder/specs/`, one Markdown file per feature. The
point of spec-based coding is that the thinking happens BEFORE the code:
the spec is complete when a different engineer could build the feature
from it without asking you anything. `procoder spec check` is the
quality controller — it blocks while sections are missing or empty, while
any `OPEN:` question is unresolved, and while acceptance criteria are
untestable.

With a feature name in the arguments, start (or resume) that spec. With
`check` or `list`, run the matching subcommand. With no arguments, run
`procoder spec list` and report.

Classify the work FIRST — and say the classification out loud so the user
can override it before the first question:

- **Spike** — a feasibility question ("can X talk to Y?"). Output is an
  answer, not kept code. State the question and the cheapest probe, get a
  nod, investigate, report. Anything built is labelled throwaway; keeping
  it is a new request that gets its own classification.
- **Bounded** — a well-scoped change to a flow that ALREADY EXISTS in
  this repo (bounded measures the repo, not your familiarity with the
  problem). No spec file: ask the questions that matter, put a short
  design in chat (approach, files touched, testing), STOP for an explicit
  yes, then implement.
- **Architectural** — new subsystems, new projects, interface changes:
  the full interview below.

In doubt between two paths, take the heavier one — and the ratchet is
one-way: hidden complexity discovered mid-task upgrades the path (stop,
say so, step up); nothing ever downgrades. The ceremony scales with the
task; the approval gate never does — no implementation before the user's
explicit yes, on any path.

The interview — this is the core of the skill:

1. `procoder spec template <name>` prints the spec shape; write it to
   the printed path.
2. Fill it by interviewing the user, one topic at a time — do NOT invent
   answers to design questions the user has not decided. Ask the right
   questions to close gaps:
   - **Problem**: what hurts today, for whom, why now? If the user gives a
     solution, ask for the problem behind it.
   - **Users**: who touches this and what does each need?
   - **In / out of scope**: propose the boundary and have the user confirm
     it — out-of-scope is written down so nobody assumes it.
   - **Constraints**: performance, compatibility, security, platform.
   - **Interfaces**: commands, APIs, file formats, UI surfaces.
   - **Data**: what is stored, where, in what shape.
   - **Edge cases**: enumerate the inputs and states that break naive
     implementations; ask about the ones the user has not mentioned.
   - **Failure modes**: for each dependency — what happens when it is
     missing, slow, or wrong?
   - **Acceptance criteria**: one `- [ ]` per observable behaviour a
     reviewer can verify. "Works well" is not a criterion; "renders with
     the network cable pulled" is.
3. Every decision you cannot close in the interview goes in as an
   `- OPEN: <question>` line under Open questions. Resolve each with the
   user, then move the decision into its proper section and delete the
   OPEN line. Do not silently decide for the user.
4. Run `procoder spec check <name>` after each pass. It names the
   remaining gaps — keep interviewing until it says COMPLETE. Do not start
   implementing while the spec is blocked.
5. When it is COMPLETE: for architectural work, write the implementation
   plan next (/procoder:plan — its own quality controller gates it);
   for smaller work, seed the task list directly from the acceptance
   criteria (`procoder todo add`, one task per coherent group — see
   /procoder:todo). Then build against the spec. If reality contradicts
   the spec mid-build, update the spec first and re-check.
