# ai-native-sdlc-map

Status: open
Created: 2026-09-03

## Question

Anthropic's AI-native SDLC playbook describes six stages, each with named
artifacts and prescribed practices. Which of them does procoder already
cover, where is it thinner than the playbook, and what is absent on
purpose?

Asked because the playbook was read as describing a new Claude Code
feature. It is not one — `intent.md` appears in no release of the CLI, and
Anthropic's own course page says in as many words that it is "a
team-created workflow convention, not a built-in Claude Code feature". What
it actually describes is a way of working, encoded as skills, `CLAUDE.md`
and `REVIEW.md`. That is procoder's own shape, which makes the comparison
worth doing properly rather than adopting one filename from it.

## What we know

Stage by stage, with what the playbook prescribes and what this repository
already has. Everything in the procoder column was checked in the tree, not
recalled.

**Stage 1 — Plan → `intent.md`.** The originator's raw ask, in their own
words, product-owner approved, committed. procoder has `analyze`
(`.procoder/analysis/`) in that slot, but it is a different document: it
asks Question / What we know / What we do not know / Options /
Recommendation. That is the work of deciding, not the record of what was
asked. **Partial.** The approval half is absent and is already designed and
parked as `approve` rights on `spec` in #248.

**Stage 2 — Design → `spec.md`, constrained by organisation skills,
concerns escalated to policy owners.** procoder has `spec` with a
controller that blocks until every section is answered and every open
question resolved — stricter than the playbook, which asks for a review
rather than a refusal. The "organisation skills" are `.procoder/`'s
`security/RULES.md`, `docs/RULES.md`, `lint/RULES.md`, read by their
domains. **Covered**, except that escalation-to-a-policy-owner is a role
routing procoder has no roles for (#248 again).

**Stage 3 — Build → `plan.md`, `CLAUDE.md`, skills, hooks, worktrees.**
procoder has `plan` with its own blocking controller, `todo` and `backlog`
for execution, `AGENTS.md` and `PRINCIPLES.md` as the standing rules,
PostToolUse hooks for format/lint/secrets, a PreToolUse hook intercepting
`git commit`, and `WORKFLOW.md` prescribing a worktree per branch.
**Covered, and stronger.** The playbook says "when Claude makes a mistake
twice, the correction goes into `CLAUDE.md`"; procoder has `LESSONS.md`
with a class, an adaptation and an unlearned check that the gate reads.

**Stage 4 — Test → verify its own work, failing test first, run
build/test/lint before reporting done, and EVALS.** procoder has `test`
(the canonical runner per ecosystem, where NOT run is never green), the
`tdd` skill (red before green, name the break each test catches, mutation
check before done), and the gate's suite leg. **Covered except evals.**
There is no evaluation concept anywhere in `commands`, `skills`, `internal`
or `docs` — checked. The playbook asks for each task to be written as an
eval, run non-interactively in CI on a schedule, with every production
incident becoming one and staying as a regression test.

**Stage 5 — Deploy → `REVIEW.md` divided into passes, Claude gives and
receives reviews, findings feed back, hooks as approval gates, CI
non-interactive, deploy through MCP, autonomy tiered by environment.**
procoder has `REVIEW.md` in exactly that shape, `/procoder:pr` and
`/procoder:merge` (every thread answered, the reflection step for anything
that escaped), findings routed into `LESSONS.md`, and `ci` for workflow
hygiene. **Covered for the review half.** Deploy through MCP and
autonomy-by-environment are absent — procoder is a pre-merge governance
tool and does not deploy anything.

**Stage 6 — Maintain → deterministic detection, `bands.yaml` response
tiers, incident diagnosis written back as an `intent.md`, an eval per
incident, scheduled security scans.** procoder has `security`
(secrets/SAST/deps), `deps`, `debt`, `maintain`, `learn`, and
`copilot-leak` — which is precisely the shape of "an external reviewer's
finding becomes tracked work", and #270 generalised it to any review
finding too large for its PR. **Mostly absent**, and most of it by design:
production monitoring, control bands and incident response are not what a
commit-time governance tool does.

## What we do not know

- **Whether evals are worth building here, and what they would assert.**
  The playbook's evals test agent BEHAVIOUR, not code. procoder governs
  agents and has no way to check that its governance produces the behaviour
  it intends — the `e2e-campaign` analysis asks a neighbouring question
  ("what breaks for somebody who is not us") and answers it with a manual
  campaign. What would resolve this: write one eval by hand for a rule that
  already exists — say, that an unformatted write produces a format finding
  the agent acts on — and see whether the assertion is meaningful or
  tautological.
- **Whether Stage 6 is in scope at all.** procoder has never claimed
  production. What would resolve this: decide whether "procoder governs the
  commit" is the boundary, and write it down, because three of the
  playbook's six stages sit outside it and that is a positioning statement
  rather than a gap.
- **Whether the intent artifact adds anything once #248 exists.** Its
  distinguishing feature is the approval, and the approval is the parked
  work. What would resolve this: revisit after team mode, not before.

## Options

- **Take the eval gap only.** It is the one concrete, in-scope practice the
  playbook prescribes and procoder lacks entirely. Costs a new domain and a
  CI cadence; buys the ability to assert that the harness produces the
  behaviour its rules describe, which nothing currently checks.
- **Take the eval gap and write down the boundary.** As above, plus a short
  statement in the docs that procoder governs up to the merge and does not
  deploy or monitor — turning three "absent" rows into a stated scope.
  Costs a paragraph; buys an answer to "why does procoder not do Stage 6"
  that is a decision rather than an omission.
- **Nothing, and record the map.** The comparison is the value: two of six
  stages are covered more strictly than the playbook asks, one is partial
  with its missing half already parked, one is missing evals, and two are
  out of scope. Leaving it recorded means the next person to read the
  playbook does not redo this.
- **Build the intent stage anyway.** Faithful to the playbook's chain, at
  the cost of a fifth document in front of code holding fields the spec
  already blocks on, with its one distinguishing feature (approval)
  unavailable until #248.

## Recommendation

**Take the eval gap and write down the boundary**, and do not build the
intent stage.

The map says procoder is not behind this playbook — it is ahead of it on
Stages 2, 3 and 5, where a controller refuses rather than a human reviews.
The single practice it prescribes that procoder has no answer to is evals,
and the reason that gap matters here more than in a normal codebase is
specific: procoder's whole product is agent behaviour, and its tests
exercise its own Go functions rather than the behaviour those functions are
meant to produce. Everything else the playbook asks for is either already
here, stricter here, parked in #248, or outside what this tool does.

The boundary is worth writing down in the same pass because three of six
stages fall outside it, and an unstated scope reads as a gap to anyone
holding the playbook next to the README.

`intent.md` is not the thing to take. Its fields — problem, proposed
outcome, affected users, constraints, open questions — are already sections
of a procoder spec, and that spec refuses to be called complete while any
of them is empty.
