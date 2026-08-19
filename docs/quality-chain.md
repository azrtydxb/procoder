# The quality chain

procoder's core belief: **the thinking happens before the code, and
"done" is a verdict a controller gives — not a feeling the agent has.**
The chain runs spec → plan → todo → gate → lessons, and every link has a
quality controller in the binary that refuses to advance until the work
is actually complete. The agent writes; the binary judges (P-CONTROL).

```
idea ──► SPEC ──► PLAN ──► TODO ──► GATE ──► merged
         │        │        │        │          │
     spec check  plan    todo     check    lessons: anything
     blocks on   check   close    blocks   that escaped becomes
     gaps and    blocks  refuses  on any   an adaptation that
     OPEN:       hollow  without  finding  closes its class
     questions   tasks   evidence
```

## Spec — close the gaps before they cost

`/procoder:spec` classifies the work out loud first — **spike** (answer a
question, keep nothing), **bounded** (a short design in chat for a flow
that already exists), or **architectural** (the full interview) — and in
doubt takes the heavier path; the ratchet only goes up. The interview
fills `.procoder/specs/<name>.md` one topic at a time: problem, users,
scope boundaries, constraints, interfaces, data, edge cases, failure
modes, acceptance criteria. Decisions the user hasn't made go in as
`OPEN:` lines — the agent never silently decides.

`procoder spec check` is the controller: it blocks while any section is
missing or empty, any `OPEN:` question is unresolved, or any acceptance
criterion is untestable ("user-friendly" is called out; "renders with
the network cable pulled" passes).

## Plan — how, exactly, for a stranger

`/procoder:plan` writes `.procoder/plans/<name>.md` from a COMPLETE spec,
for an engineer with zero context: Goal, Architecture, Constraints
copied verbatim, and `## Task N:` blocks each carrying `Files:`,
`Interfaces:` (the exact signatures neighbouring tasks use — a task's
implementer sees only their own task), and test-first checkbox steps.

`procoder plan check` blocks the placeholder failure modes: "TBD",
"handle edge cases", "similar to Task N" (repeat the code — tasks are
read out of order), empty sections, tasks without files or steps. A plan
is written, not promised.

## Todo — done means evidence

`/procoder:todo` tracks execution in `.procoder/todo/`, one file per
task with a real description, acceptance-criteria checkboxes, and an
`## Evidence` section. What counts as evidence is strict: fresh runs
only, the red-green proof for regression tests, and a subagent's "done"
is a claim until verified in the diff.

`procoder todo close` refuses — naming exactly what is missing — until
every criterion is checked, the evidence records what was run and what
it proved, and **the commit gate itself is clean**. The binary runs the
real gate as the final criterion: a task cannot close on a broken tree.

## Gate — one code path, no disagreement

`procoder check` is the same collection CI runs and `procoder git`
prints: formatting, hygiene, secrets, lint, ci/infra/docs health, the
agent-layer drift guards, and the change's blast radius from the index.
Unchecked counts as failing. There is no side door.

## Lessons — escapes close their class

Before a PR exists, `/procoder:pr` runs the **pre-PR self-review**: a
fresh-context reviewer reads the diff against
`.procoder/github/REVIEW.md`. What still escapes to a downstream
reviewer triggers `/procoder:merge`'s **reflection step**: name the
layer that should have caught it, adapt that layer in the same PR, and
record it in `.procoder/github/LESSONS.md`. `procoder lessons` flags any
entry with no adaptation as UNLEARNED (exit 1) — recorded is not
learned. The rubric grows from real escapes, so the same class cannot
escape twice.

## Why refusal, not advice

Every controller in the chain answers with a refusal that names the gap,
never a warning the agent can wave through. Advice gets rationalized
under pressure; a named, blocking gap gets fixed. That is the difference
between a checklist and a gate — and it is why the chain's verdicts
("COMPLETE", "closed", "clean") actually mean something when you read
them in a log.
