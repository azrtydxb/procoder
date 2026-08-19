# backlog

Status: draft

## Problem

The quality chain ends at the todo list: spec → plan → todo works for a
feature, but a larger project built spec-first has nowhere to live. There
is no way to group work into epics under milestones, no user-story layer
between a spec's acceptance criteria and execution, and no sprint
discipline — so on multi-week projects the agent (and Pascal) lose the
map: what is committed now, what waits in the backlog, what an unfinished
sprint actually left behind. Today that structure lives in heads or in
external tools procoder cannot gate.

## Users

- **Pascal (and any developer running procoder)** — needs to lay out a
  project as milestones → epics → stories, pull stories into a sprint,
  and see one board that says where everything stands.
- **The agent** — needs machine-checkable state: which stories are
  committed to the active sprint, which controller refusals block a
  close, and how a completed spec decomposes into an epic with stories.
- **The quality chain** — the backlog is a fourth link: spec (design) →
  plan (how) → backlog (what, in what order, at what altitude) with the
  same refusing-controller discipline as spec/plan/todo.

## In scope

- A backlog domain under `.procoder/backlog/` with three levels:
  **milestone → epic → story**. The story is the execution unit of
  spec-based work and carries description, acceptance criteria, and
  evidence — the same rigor as a todo task.
- The existing **todo domain stays untouched**: it remains the standalone
  list for tasks NOT related to spec-based development. Nothing about
  `procoder todo` changes.
- `procoder backlog` subcommands: `milestone <title>`,
  `epic <title> [--milestone <id>]`, `story <title> --epic <id>`,
  `seed <spec> [--milestone <id>]`, `list`, `board`,
  `close <story|epic|milestone> <id>`.
- `procoder sprint` subcommands: `open <goal>`, `pull <story-id>...`,
  `carry <story-id> <reason>`, `status`, `close`.
- **Spec seeding**: `backlog seed <spec>` decomposes a COMPLETE spec into
  one epic plus one story per acceptance criterion, printed for the agent
  to review and write (P-CONTROL). The epic records the source spec and a
  content fingerprint; `board` flags the epic when the spec has changed
  since seeding (drift), and when the spec file is missing.
- **Refusing controllers** (the lean/agile discipline, encoded):
  - Story close refuses until description is real, every acceptance
    criterion is checked, evidence is recorded, and the gate is clean —
    full rigor, identical in spirit to todo close.
  - Epic close refuses while any of its stories is not done, and warns
    on spec drift in the refusal output.
  - Milestone close refuses while any of its epics is open.
  - Sprint open refuses while another sprint is active (WIP limit: one).
  - Sprint close refuses while a committed story is neither done nor
    explicitly carried back to the backlog with a reason — unfinished
    work is visible, never silent. The close writes a summary (committed,
    done, carried counts) into the sprint file.
- Scope-boxed sprints: a sprint is a goal plus the stories pulled into
  it, opened and closed explicitly. `Created:` dates are recorded for
  humans; no calendar enforcement.
- No estimation: sprint reports count stories, never points.
- Documentation, usage text, skill files (`commands/backlog.md`,
  `commands/sprint.md` + OpenCode twins), and tests, per the repo's
  rot guards (docs.Commands, usage pin test, twin parity test).

## Out of scope

- Story points, t-shirt sizes, velocity charts, burndown — estimation is
  deliberately absent (decided in the interview: counting stories only).
- Calendar timeboxing: no start/end date enforcement, no overrun alarms.
- Multiple concurrent sprints (one active sprint, full stop — not even
  one per milestone).
- Any change to the todo domain, or migration of todo tasks into the
  backlog.
- Kanban WIP limits below the sprint level (per-story in-progress caps).
- Multi-repo or team-server backlogs: state is this repository's
  `.procoder/backlog/`, committed with the repo like every other
  procoder artifact.
- A TUI/web board: `board` is plain text on stdout.

## Constraints

- Pure Go stdlib inside the one binary; no new dependencies.
- P-CONTROL: creation subcommands PRINT file content for the agent to
  review and write; the binary writes only procoder-owned state files
  under `.procoder/backlog/` for verified state transitions (close,
  pull, carry) — the precedent is todo close rewriting `Status:`.
- Honesty rule: an unreadable backlog file is listed as `unreadable`,
  never skipped; a refusal names every missing thing, not just the first.
- Cross-platform (darwin/linux/windows), no shell-outs except the
  existing gate run for story close.
- The gate check reuses `gate.Run` exactly as todo close does.
- File format is plain Markdown with the same `Status:` /
  `## Section` conventions as todo, so agents edit files directly.

## Interfaces

- `procoder backlog milestone <title>` — prints
  `.procoder/backlog/milestones/<slug>.md` content (Goal section,
  `Status: open`, `Created:`).
- `procoder backlog epic <title> [--milestone <id>]` — prints
  `.procoder/backlog/epics/<slug>.md` with optional `Milestone:` field,
  optional `Spec:` field, Description section.
- `procoder backlog story <title> --epic <id>` — prints
  `.procoder/backlog/stories/<slug>.md` with `Epic:` field,
  `Sprint: -` field, Description / Acceptance criteria / Evidence
  sections (todo-task shape).
- `procoder backlog seed <spec> [--milestone <id>]` — refuses unless
  `spec check <spec>` is COMPLETE; prints the epic file (with
  `Spec: <name> @ <fingerprint>`) and one story file per acceptance
  criterion, each labeled with its target path. Refuses if the epic slug
  already exists, pointing at it.
- `procoder backlog list` — flat listing: every milestone, epic, story
  with status; open first.
- `procoder backlog board` — the tree: milestones → their epics → their
  stories with status markers; orphan epics/stories (parent missing or
  deleted) under an ORPHANS heading; spec-drift flags on epics; one
  summary line (counts by status, active sprint name if any).
- `procoder backlog close story|epic|milestone <id>` — the controllers;
  exit 0 closed, 1 refused with reasons, 2 usage/not-found.
- `procoder sprint open <goal words...>` — refuses if active sprint
  exists; prints `.procoder/backlog/sprints/<NNN>-<slug>.md`
  (`Status: active`, Goal).
- `procoder sprint pull <story-id>...` — sets `Sprint: <id>` in each
  story (binary write, verified transition); refuses stories already in
  a sprint, already done, or nonexistent — per story, processing the
  rest.
- `procoder sprint carry <story-id> <reason words...>` — clears the
  story's `Sprint:` back to `-` and appends a `Carried:` line naming the
  sprint and reason; refuses without a reason.
- `procoder sprint status` — active sprint: goal, each committed story
  with status, done/total count, carried count.
- `procoder sprint close` — the controller; on success rewrites
  `Status: closed <date>` plus a `## Result` summary into the sprint
  file.
- Usage text gains `backlog` and `sprint` entries; `docs.Commands`,
  `commands/backlog.md`, `commands/sprint.md`, OpenCode twins, and the
  docs site follow.

## Data

- `.procoder/backlog/milestones/<slug>.md` — `# <title>`,
  `Status: open|done`, `Created: <date>`, `## Goal`.
- `.procoder/backlog/epics/<slug>.md` — `# <title>`,
  `Status: open|done`, `Created:`, optional `Milestone: <slug>`,
  optional `Spec: <name> @ <12-hex fingerprint>` (SHA-1 prefix of the
  spec file content at seed time), `## Description`.
- `.procoder/backlog/stories/<slug>.md` — `# <title>`,
  `Status: open|done`, `Created:`, `Epic: <slug>`, `Sprint: <id or ->`,
  optional `Carried: <sprint-id> — <reason>` lines, plus Description,
  Acceptance-criteria, and Evidence sections (the todo-task shape).
- `.procoder/backlog/sprints/<NNN>-<slug>.md` — `# <goal>`,
  `Status: active|closed <date>`, `Created:`, a Goal section, and a
  Result section written at close (committed N, done N, carried N with
  story ids).
- Ownership: all files are procoder-owned state, committed with the
  repository (not gitignored — unlike the derived index, the backlog IS
  the project record). Parent links are child → parent references by
  slug; the board assembles the tree by reading everything.
- Story slugs are `<date>-<slug>` like todo ids; milestone/epic slugs
  are plain slugified titles; sprint ids are zero-padded sequence +
  goal slug (`001-auth-mvp`).

## Edge cases

- `seed` on a spec that is not COMPLETE → refuse, print the spec-check
  gaps, exit 1.
- `seed` on a spec with zero acceptance criteria (or only placeholder
  `...`) → refuse: an epic with no stories is not a decomposition.
- `seed` when the epic slug already exists → refuse and name the
  existing epic (re-seeding after spec changes is a manual decision).
- Two titles slugifying identically → creation subcommands refuse when
  the target file already exists rather than printing an overwrite.
- Story whose `Epic:` names a missing epic (deleted by hand) → shown
  under ORPHANS on the board; epic close unaffected (it only counts
  stories that reference it).
- `sprint pull` of a story that is done, already in a sprint, or
  missing → that story is refused with its reason; remaining arguments
  still process; exit 1 if any refused.
- `sprint carry` of a story not in the active sprint → refuse.
- `sprint close` with zero pulled stories → allowed (an empty sprint
  closes with a zero summary) — opening it was the mistake, holding it
  hostage helps nobody.
- `sprint open` while a sprint file is unreadable → refuse and say so:
  an unreadable sprint might be active; never silently open a second.
- Story close for an id that exists but is already done → say
  "already done", exit 0 (idempotent, like todo).
- Ids containing path separators or `..` → refused (same guard as
  `todo.File`).
- Hand-edited `Status:` values outside the known set → listed verbatim
  in list/board; controllers treat anything other than `done`/`closed`
  as open (refusals stay conservative).
- Backlog directory absent → list/board report an empty backlog and
  point at `backlog milestone`/`seed`; nothing errors.

## Failure modes

- **Gate unavailable/failing** during story close → the refusal says
  "the gate is not clean" exactly as todo close does; a gate that cannot
  run counts as not clean (honesty rule).
- **Spec file missing** for an epic's `Spec:` reference → board flags
  `spec missing`, epic close warns but is not blocked by it (stories are
  the contract; the fingerprint is traceability).
- **Unreadable file** anywhere under `.procoder/backlog/` → surfaces as
  status `unreadable` in list/board; controllers refuse to close a
  parent whose child is unreadable (unknown ≠ done).
- **Write failure** on a verified transition (close/pull/carry) → the
  error is printed verbatim and the exit code is 2; no partial rewrite
  (whole-file write, same as todo).
- **Concurrent hand-edits**: last write wins, files are plain Markdown
  in git — the repo history is the recovery path; no locking.

## Acceptance criteria

- [ ] `procoder backlog seed <complete-spec>` prints one epic file and
      one story file per acceptance criterion, each with its target
      path; on an incomplete spec it refuses and prints the gaps.
- [ ] `procoder backlog board` renders milestone → epic → story with
      statuses, shows an epic's spec-drift flag after the source spec
      file changes, and lists a story with a deleted epic under ORPHANS.
- [ ] `procoder backlog close story <id>` refuses with a list naming
      each gap (empty description, placeholder criteria, unchecked
      boxes, empty evidence, dirty gate) and closes only when all pass —
      verified by a test that walks a story from refused to closed.
- [ ] `procoder backlog close epic <id>` refuses while any story
      referencing it is open and closes when all are done; milestone
      close does the same over its epics.
- [ ] `procoder sprint open` refuses while another sprint is active;
      exactly one sprint can be active, verified by test.
- [ ] `procoder sprint pull` sets `Sprint:` in the story file; pulling a
      done, missing, or already-pulled story refuses that story with a
      reason and exit 1 while other arguments still process.
- [ ] `procoder sprint close` refuses while a committed story is
      neither done nor carried; after `sprint carry <id> <reason>` the
      story is back in the backlog carrying a `Carried:` line, and close
      succeeds writing committed/done/carried counts into `## Result`.
- [ ] The todo domain's behaviour is unchanged (its tests pass
      untouched), and todo remains documented as the standalone list for
      non-spec work.
- [ ] Usage text lists `backlog` and `sprint`; `docs.Commands`, the docs
      site, `commands/backlog.md`, `commands/sprint.md`, and their
      OpenCode twins exist — the usage-pin, CommandCoverage, and twin
      parity tests all pass.
- [ ] `go test ./...` passes on a tree where every new controller path
      (refuse and succeed) has at least one test.

## Open questions

<!-- none — all interview decisions are recorded above -->
