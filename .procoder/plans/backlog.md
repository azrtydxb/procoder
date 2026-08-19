# backlog — implementation plan

Status: draft
Spec: .procoder/specs/backlog.md

## Goal

Ship the backlog/sprint domain — milestone → epic → story with refusing
controllers and scope-boxed sprints — as `procoder backlog` and
`procoder sprint`, released as 0.26.0.

## Architecture

One new package `internal/backlog` owns items (milestones, epics,
stories), sprints, seeding, and every controller; `cmd/procoder/main.go`
gains two dispatch functions in the existing switch style. Files under
`.procoder/backlog/{milestones,epics,stories,sprints}/` follow the todo
Markdown conventions (`Status:` line, `## Section` bodies), so parsing
helpers mirror `internal/todo/todo.go`. Creation prints, verified
transitions rewrite whole files — P-CONTROL exactly as todo.

## Constraints

- Pure Go stdlib; no new dependencies.
- P-CONTROL: `milestone`/`epic`/`story`/`seed`/`sprint open` PRINT file
  content and the target path; only `close`/`pull`/`carry` rewrite
  files, and only under `.procoder/backlog/`.
- Honesty: unreadable files surface as `unreadable`; every refusal lists
  ALL gaps found, not the first; controllers treat unknown statuses as
  open.
- Id guard from `todo.File` semantics: refuse ids with separators or
  `..`.
- Exit codes: 0 success/idempotent, 1 controller refusal, 2 usage or
  not-found.
- Statuses: items use `Status: open` / `Status: done <date>`; sprints
  use `Status: active` / `Status: closed <date>`.
- Story files carry `Epic: <slug>` and `Sprint: <id or ->` header lines
  directly under `Created:`.
- Every touched doc/rot-guard: usage text, `docs.Commands`,
  `docs/commands.md`, `docs/domains.md` (quality-chain mention),
  `docs/quality-chain.md`, `commands/backlog.md`, `commands/sprint.md`,
  OpenCode twins, CHANGELOG, version bump to 0.26.0 in the nine
  manifest/docs files, dist rebuild for five platforms.

## Task 1: items — types, parsing, creation templates

Files: internal/backlog/items.go, internal/backlog/items_test.go
Interfaces: `const Dir = ".procoder/backlog"`; `type Item struct {ID,
Title, Status, Kind, Milestone, Epic, Sprint, Spec, Path string}`;
`func LoadAll(root string) ([]Item, error)` (kinds: milestone, epic,
story, sprint); `func ItemFile(root, kind, id string) (string, error)`
(id guard); `func Milestone(root, title string, out func(string)) int`;
`func Epic(root, title, milestone string, out func(string)) int`;
`func Story(root, title, epic string, out func(string)) int`; shared
helpers `section`, `stripComments`, `slugify` (copied from todo — the
two domains stay independent per the spec, todo untouched).

- [ ] Define the three creation templates: milestone (`# title`,
      `Status: open`, `Created:`, `## Goal`); epic (adds optional
      `Milestone:` and `Spec:` header lines, `## Description`); story
      (`Epic:`, `Sprint: -` headers plus the todo-task trio of
      Description / Acceptance criteria / Evidence sections with the
      same guidance comments).
- [ ] `Milestone`/`Epic`/`Story` slugify the title (story ids prefixed
      `YYYYMMDD-` like todo), REFUSE with exit 2 when the target file
      already exists (slug collision, spec edge case), refuse an empty
      slug, refuse `Story` without `--epic`; each prints
      `== write this to <path>:` then the filled template.
- [ ] `LoadAll` walks the four subdirectories, parses `Status:`,
      `Milestone:`, `Epic:`, `Sprint:`, `Spec:` and the `# ` title from
      each `.md`; an unreadable file becomes `Status: unreadable` with
      the error as Title; missing directories yield an empty slice, no
      error.
- [ ] Tests: template refusal on existing file; story requires epic;
      LoadAll on a hand-built tree returns every kind with correct
      fields; unreadable file (chmod 000, skipped on Windows) surfaces
      as unreadable; empty/missing `.procoder/backlog` returns empty.

## Task 2: seeding an epic from a COMPLETE spec

Files: internal/backlog/seed.go, internal/backlog/seed_test.go
Interfaces: `func Seed(root, specName, milestone string,
out func(string)) int`; consumes `spec.Check(root, name, discard)` for
completeness and `checkboxRe`-style parsing of the spec's Acceptance
criteria section; produces epic + story file printouts using Task 1
templates; fingerprint helper `func fingerprint(b []byte) string`
(sha1, first 12 hex).

- [ ] `Seed` refuses (exit 1) when `spec check` is not COMPLETE,
      replaying the checker's gap lines; refuses when the spec has zero
      non-placeholder `- [ ]` criteria ("an epic with no stories is not
      a decomposition"); refuses (exit 2) when the epic slug (the spec
      name) already exists, naming the existing file.
- [ ] On success prints: one epic file with
      `Spec: <name> @ <fingerprint>` (+ `Milestone:` when given), then
      one story file per criterion — story title is the criterion text
      (trimmed to one line), `Epic:` set to the epic slug — each block
      preceded by its `== write this to <path>:` header.
- [ ] Tests: incomplete spec refused with gaps shown; complete fixture
      spec seeds N stories with correct Epic links and fingerprint;
      placeholder-only criteria refused; existing epic refused.

## Task 3: the close controllers — story, epic, milestone

Files: internal/backlog/close.go, internal/backlog/close_test.go
Interfaces: `func CloseStory(root, id string, gateClean func() bool,
out func(string)) int`; `func CloseEpic(root, id string,
out func(string)) int`; `func CloseMilestone(root, id string,
out func(string)) int`. main.go passes the same
`func() bool { return gate.Run(nil, root, io.Discard) == 0 }` that todo
close uses.

- [ ] `CloseStory` mirrors `todo.Close` verbatim in rigor: empty
      description, placeholder criteria, no checked box, N unchecked
      boxes, empty evidence, dirty gate — ALL findings listed in one
      refusal; already-done exits 0; success rewrites
      `Status: done <date>`.
- [ ] `CloseEpic` loads all stories whose `Epic:` is this id: refuses
      naming each not-done story (unreadable counts as not done, spec
      failure-mode rule); appends a `spec drift` warning line to the
      refusal/success output when the epic's `Spec:` fingerprint no
      longer matches the current spec file (or the spec file is
      missing) — drift warns, never blocks; success rewrites
      `Status: done <date>`.
- [ ] `CloseMilestone` refuses naming each open epic that references
      it; success rewrites `Status: done <date>`.
- [ ] Tests: a story walked refused → closed (checking every refusal
      line appears); epic refused while story open, closes after,
      drift warning fires when the spec file changes; milestone refused
      then closed; id traversal guard refused.

## Task 4: sprints — open, pull, carry, status, close

Files: internal/backlog/sprint.go, internal/backlog/sprint_test.go
Interfaces: `func SprintOpen(root, goal string, out func(string)) int`;
`func SprintPull(root string, ids []string, out func(string)) int`;
`func SprintCarry(root, id, reason string, out func(string)) int`;
`func SprintStatus(root string, out func(string)) int`;
`func SprintClose(root string, out func(string)) int`;
`func activeSprint(root string) (Item, bool, error)` (error on any
unreadable sprint file — never silently open a second).

- [ ] `SprintOpen` refuses (exit 1) while a sprint is active or any
      sprint file is unreadable; otherwise prints the sprint file
      (`NNN-<goalslug>.md`, NNN = max existing + 1 zero-padded,
      `Status: active`, `## Goal`) — creation prints, per P-CONTROL.
- [ ] `SprintPull` requires an active sprint; per story id: missing,
      done, or `Sprint:` already set → refused with reason, remaining
      ids still process (exit 1 if any refused); accepted stories get
      `Sprint: <id>` rewritten in place.
- [ ] `SprintCarry` requires the story to be in the active sprint and a
      non-empty reason; rewrites `Sprint: -` and appends
      `Carried: <sprint-id> — <reason>` under the header block.
- [ ] `SprintStatus` prints goal, each committed story with status,
      `done/total` count and carried count; no active sprint → says so,
      exit 1.
- [ ] `SprintClose` refuses naming each committed story neither done
      nor carried; success rewrites `Status: closed <date>` and appends
      `## Result` with committed/done/carried counts and story ids
      (zero-story sprints close with a zero summary).
- [ ] Tests: second open refused; pull mixed batch (one good, one done,
      one missing) exits 1 but pulls the good one; carry then close
      succeeds with correct Result counts; close refused while a story
      is open; unreadable sprint file blocks open.

## Task 5: list and board

Files: internal/backlog/board.go, internal/backlog/board_test.go
Interfaces: `func List(root string, out func(string)) int`;
`func Board(root string, out func(string)) int`; both consume
`LoadAll`; Board also reads spec files to evaluate drift via
`fingerprint`.

- [ ] `List` prints every item as `  [status]  kind  id  title`, open
      first then by id; empty backlog says
      "no backlog — `procoder backlog milestone <title>` or
      `procoder backlog seed <spec>` starts one", exit 0.
- [ ] `Board` prints milestones with their epics nested, stories under
      each epic with status marks (`[ ]` open, `[x]` done,
      `[!]` unreadable) and `→ sprint <id>` tags on committed stories;
      epics without a milestone under `(no milestone)`; stories whose
      epic is missing under `ORPHANS`; epic lines flagged
      `⚠ spec drift` / `⚠ spec missing` when applicable; final summary
      line with counts by status plus the active sprint id if any.
- [ ] Tests: golden-ish assertions on a built tree covering nesting,
      orphan, drift flag, and the summary line.

## Task 6: CLI dispatch, usage, docs, skills, release

Files: cmd/procoder/main.go, internal/docs/coverage.go,
docs/commands.md, docs/domains.md, docs/quality-chain.md,
commands/backlog.md, commands/sprint.md, .opencode/command/backlog.md,
.opencode/command/sprint.md, CHANGELOG.md, README.md, the nine
version-bearing manifests, dist/* (rebuilt)
Interfaces: `func backlogCmd(args []string) int` and
`func sprintCmd(args []string) int` in main.go, wired as
`case "backlog":` / `case "sprint":` (alphabetical order in usage);
`docs.Commands` gains "backlog" and "sprint".

- [ ] Dispatch: `backlog milestone|epic|story|seed|list|board|close`
      (with `--milestone`/`--epic` flag parsing in the loop style of
      `index rename --at`) and `sprint open|pull|carry|status|close`;
      wrong arity prints usage, exit 2; usage text gains both commands
      with one-line descriptions per subcommand group.
- [ ] `docs.Commands` += "backlog", "sprint" (alphabetical) — the
      usage-pin test enforces the pairing.
- [ ] Skill files `commands/backlog.md` and `commands/sprint.md` written
      in the house voice (launcher invocations, the controller-refusal
      guidance, todo's independence stated); OpenCode twins generated by
      the established substitution rule; twin-parity test passes.
- [ ] Docs: `docs/commands.md` gains both command sections;
      `docs/quality-chain.md` and `docs/domains.md` present the backlog
      as the fourth link (spec → plan → backlog, todo standalone);
      README's quality-chain paragraph mentions milestones/epics/
      stories/sprints; CHANGELOG 0.26.0 entry.
- [ ] Release: version 0.26.0 across plugin.yaml, package.json,
      gemini-extension.json, README.md, .claude-plugin/plugin.json,
      docs/index.md, .github/plugin/{marketplace,plugin}.json,
      .codex-plugin/plugin.json; dist rebuilt for the five platforms
      with `-X main.version=0.26.0`; `go test ./...`, gate, docs report
      all green.

## Task 7: end-to-end verification on a live fixture

Files: none new (scratch repo under the session scratchpad)
Interfaces: consumes only the shipped binary.

- [ ] In a scratch git repo: write a COMPLETE spec, `backlog seed` it,
      write the printed epic + stories, `sprint open` + `pull` two
      stories, close one story properly (criteria checked, evidence,
      clean gate), `sprint carry` the other with a reason,
      `sprint close`, and `backlog board` — capturing each command's
      output as evidence that every controller refused and passed where
      the spec says it must.
