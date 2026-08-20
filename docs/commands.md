# Command reference

**Reference.** Two ways to reach the same engine, and one of them is the
way in.

## Run the `/procoder:` commands

**This is the interface.** Each one is a skill: it runs the binary, reads
the output, and knows what to do with it — which checks to re-run, what
counts as evidence, when to refuse. Typing the binary yourself gets you
the numbers without the judgment that surrounds them.

| Command              | What it does                                                                                                                                                                 |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `/procoder:adr`      | Architecture decision records: durable decisions with their date, context, and consequences — supersede, never rewrite.                                                      |
| `/procoder:agents`   | Keep the universal agent layer in sync: per-host rule files derived from AGENTS.md, with drift blocking the gate.                                                            |
| `/procoder:audit`    | Onboard an existing codebase: every domain's checks over the whole tree, then a triaged plan to bring it in line.                                                            |
| `/procoder:backlog`  | The project layer: milestones, epics, and user stories with refusing controllers — spec-seeded, sprint-ready.                                                                |
| `/procoder:check`    | Run the formatting gate over the changed files, as a commit or CI would.                                                                                                     |
| `/procoder:ci`       | CI/CD hygiene and run health: pinned actions, timeouts, concurrency, tests — plus the latest runs via gh.                                                                    |
| `/procoder:debug`    | Systematic debugging: root cause before any fix, one hypothesis at a time, and a three-strikes rule that questions the architecture instead of stacking patches.             |
| `/procoder:deps`     | The dependency freshness report: what is behind, by how much, per ecosystem — judgment stays yours.                                                                          |
| `/procoder:docs`     | The documentation report: references, diagrams, drift, API docs, badges, README structure, links, Pages.                                                                     |
| `/procoder:doctor`   | Report which formatters this repository needs, which are installed, and how to install the rest.                                                                             |
| `/procoder:env`      | What changed in the project's environment since you last synced: dependencies, migrations, new env keys.                                                                     |
| `/procoder:format`   | Show the formatted result for files, so you can review and write it.                                                                                                         |
| `/procoder:git`      | The pre-finish status: branch, hygiene findings, message checks, workflow lint, template state.                                                                              |
| `/procoder:index`    | The code index: build it, then find, search, refs, outline, and impact instead of grepping blind.                                                                            |
| `/procoder:infra`    | DevOps hygiene where the files exist: Dockerfiles, Terraform, Kubernetes manifests, Helm charts.                                                                             |
| `/procoder:init`     | Install the formatters this repository needs, with every command visible before it runs.                                                                                     |
| `/procoder:lint`     | The canonical linter per ecosystem over your changes: findings are diagnoses you judge, fix, or explain.                                                                     |
| `/procoder:maintain` | The maintainability report: dead-code candidates, complexity, function length — judgment calls you decide on.                                                                |
| `/procoder:merge`    | Finish a PR properly: every check green, every review addressed — human and bot — then merge and clean up.                                                                   |
| `/procoder:perf`     | The performance discipline: measure before touching, benchmark what matters, prove regressions and fixes with numbers.                                                       |
| `/procoder:plan`     | Turn an approved spec into an implementation plan an engineer with zero context could execute — with a quality controller that blocks placeholders and hollow tasks.         |
| `/procoder:pr`       | Prepare and open a pull request the senior way: gate, template, scrubbed, everything visible.                                                                                |
| `/procoder:release`  | The pre-tag controller: version sync, changelog, clean tree, gate, and suite — every failure listed, the tag printed, never run.                                             |
| `/procoder:run`      | How to run this project: the launch commands it declares, with the file that declared each.                                                                                  |
| `/procoder:security` | The security pass: secrets (blocking), SAST, dependency vulns — plus the index's entry points to review from.                                                                |
| `/procoder:simplify` | The over-engineering review: five tags (delete, stdlib, native, yagni, shrink), a mandatory replacement per finding, and a real null result when there is nothing to cut.    |
| `/procoder:spec`     | Spec-first design: a gap-closing interview that produces a complete spec, with a quality controller that blocks until every section is answered and every question resolved. |
| `/procoder:sprint`   | Scope-boxed sprints over the backlog: one active sprint, explicit carry-over, a close that refuses to hide unfinished work.                                                  |
| `/procoder:status`   | The state of play, computed fresh: branch, dirty files, the active sprint, open work, index freshness.                                                                       |
| `/procoder:tdd`      | Test-driven development with tests that actually catch breaks: red before green, name the break each test catches, and the mutation check before done.                       |
| `/procoder:test`     | Run the repository's actual test suite: every ecosystem's canonical runner — NOT run is never green.                                                                         |
| `/procoder:todo`     | The quality-gated task list: tasks with real descriptions, testable acceptance criteria, and evidence — a task only closes when the controller agrees it is done.            |
| `/procoder:update`   | Update the procoder plugin from the marketplace and verify the new version end to end.                                                                                       |

## The binary underneath

Everything below is what those skills call. Reach for it directly only
when you have to:

- your agent is not Claude Code — see
  [Install without the plugin](how-to-install-manually.md)
- you are scripting CI, where `procoder check` is the whole point
- you are debugging Procoder itself

It is the same binary and the same rules either way; what you give up is
the skill's judgment about what to do with the answer. **Prefer the
slash command.**

The binary only ever computes and reports — the agent (or you) acts on
the results (P-CONTROL).

### Onboarding

#### `procoder audit`

The onboarding sweep for a repository Procoder has not governed before:
every domain's checks over the WHOLE tracked tree — formatting verdicts,
hygiene, secrets, lint — aggregated into one scorecard with a triage
order. Exit 1 while the repository would fail the gate; the
`/procoder:audit` skill drives the fixing.

### Everyday commands

#### `procoder check [paths...]`

The commit gate. Over the changed files (or the given paths): formatting
(unformatted and unchecked both fail), git hygiene (conflict markers, junk
and caches, oversized files, AI-attribution lines — all blocking), workflow
lint, CI hygiene, infrastructure hygiene, lint findings, secrets (blocking),
docs checks, and the change's blast radius from the index. Exit 1 on any
blocking finding.

#### `procoder git`

The pre-finish status: branch vs default, changed-file count, template
presence and registration, and every hygiene finding — the same rules as
`check`, shared through one code path so they can never disagree.

#### `procoder format <files...>`

Prints each file's formatted result (gofmt, ruff, prettier, rustfmt,
clang-format, shfmt — the project's config always wins) so it can be
reviewed and written. Never touches the file.

#### `procoder lint [--types] [paths...]`

The canonical linter per ecosystem: golangci-lint (Go), ruff check
(Python), shellcheck (shell), eslint (JS/TS — configless plain JavaScript
gets the built-in-rules Procoder baseline; configless TypeScript is out of
scope), cargo clippy (Rust), ktlint (Kotlin), swiftlint (Swift), rubocop
(Ruby), and checkstyle (Java, google_checks baseline). Go repositories without a golangci config get Procoder's curated
baseline (standard set plus gosec, gocritic, errorlint, unparam,
copyloopvar, nilerr) — the repo's own golangci config always wins,
whichever of `.golangci.yml`/`.yaml`/`.toml`/`.json` it uses. Report
by default; `[lint] policy = "block"` makes findings block.

`--types` adds the type-checker where the canonical linter does not
compile the code: `tsc --noEmit` for TypeScript (grouped under each
file's nearest tsconfig — without one the file is declared out of scope,
never silently skipped) and pyright for Python. Go and Rust need no
flag: golangci-lint and clippy already compile what they lint.

#### `procoder backlog <sub>`

The project layer under `.procoder/backlog/`: **milestones → epics →
user stories**, with the story as the execution unit of spec-based work
(the todo list stays standalone for everything else).

- `milestone <title>` / `epic <title> [--milestone <id>]` /
  `story <title> --epic <id>` — print each file for the agent to
  review and write; slug collisions refuse rather than overwrite.
- `bug <title> [--epic <id>] [--severity s1|s2|s3|s4]` — a defect is a
  story with `Type: bug` and a severity (default s3): the description
  prompts for reproduction steps, the criteria are pre-seeded with the
  non-negotiable regression test, and closing without a severity is
  refused. The board marks open bugs with their severity.
- `seed <spec> [--milestone <id>]` — decompose a COMPLETE spec into an
  epic plus one story per acceptance criterion. The epic records the
  spec name and a fingerprint of its acceptance criteria — the
  contract, so rewrapping prose never reads as drift; an incomplete
  spec is refused
  with the spec checker's gaps replayed.
- `list` / `board` — the flat listing, and the tree with statuses,
  sprint tags, spec-drift flags (`⚠ spec drift` / `⚠ spec missing` /
  `⚠ spec not seeded`), orphans, and a summary line. An open story
  missing a section the close controller reads is flagged there rather
  than at close. The backlog is versioned like the code, so the board
  answers about the current checkout: its last line names the branch it
  read and counts the open stories the default branch holds that this
  one cannot see. The two are never merged — whose status wins when
  both branches carry a story is a decision, not a default.
- `close story <id>...` — refuses until the description is real, every
  acceptance criterion is checked, evidence is recorded, and the gate
  is clean — todo-close rigor, applied to stories. Several ids share
  ONE gate and suite run: the tree is what they judge, so asking per
  story only repeats the answer. An incomplete story is refused by name
  while the rest still close.
- `close epic <id>` / `close milestone <id>` — refuse while any child
  is open; epic close warns on spec drift (never blocks on it).

#### `procoder sprint <sub>`

Scope-boxed sprints over the backlog — a goal plus the stories pulled
into it, no story points, no calendar enforcement.

- `open <goal>` — refuses while another sprint is active (one active
  sprint is the WIP limit); prints the sprint file.
- `pull <story-id>...` — commits stories to the active sprint; done,
  missing, or already-committed stories are refused individually while
  the rest still pull.
- `carry <story-id> <reason>` — returns an unfinished story to the
  backlog with the reason recorded in the story file; no reason, no
  carry.
- `status` — goal, committed stories, done/total and carried counts.
- `close` — refuses while a committed story is neither done nor
  carried; on success the sprint file gains a Result section with
  committed/done/carried counts, plus a Retro scaffold (what slowed
  us, what we change, one adaptation worth keeping).
- The retro is the price of the next sprint: `open` refuses while the
  last closed sprint's Retro is empty. A repo opts out with
  `[sprint] retro = "off"` in config.toml.

#### `procoder release [<version>]`

The pre-tag controller: verifies in one pass that every file in
`[release] files` (config.toml) carries the version, CHANGELOG.md has
the `## <version>` entry, the working tree is clean (untracked
included), the gate is clean, and the suite is green under
`[test] policy`. Every failure is listed together; on success the
`git tag` command is printed for the agent to run — the binary tags
nothing. Without `[release] files` the version-sync leg says
it verified nothing. Bare `procoder release` reads the newest changelog
version and checks that.

#### `procoder adr <sub>`

Architecture decision records under `.procoder/adr/`, numbered and
immutable — a changed mind writes a new record and supersedes the old.
`new <title>` prints the next-numbered record (Context / Decision /
Consequences); `list` shows proposed first; `check` refuses hollow
records, unknown statuses, dangling supersede references, and
duplicated numbers. The audit sweep includes these findings.

#### `procoder deps`

The freshness report: outdated dependencies per ecosystem via each
one's native tool — `go list -u -m`, `npm outdated`, cargo-outdated and
pip where available — capped, summarized, report-only. Licenses report
where a tool exists (go-licenses for Go) and answer NOT checked
everywhere else. A missing optional tool is information; a
tool that errored is a failure.

#### `procoder bench [--save]`

The Go benchmarks (`go test -bench . -benchmem`), compared against the
committed baseline in `.procoder/bench/baseline.txt`: per-benchmark
ns/op and B/op deltas, regressions beyond `[bench] threshold` (default
10%) marked and exiting 1, new and vanished benchmarks listed. `--save`
records a new baseline — a decision, taken deliberately. Results are
single-run and machine-local; a baseline from another GOOS/GOARCH
compares with a warning. Go only in this version, said out loud.

#### `procoder test [--coverage] [--name <pattern>] [paths...]`

The repository's actual test suite, run by every detected ecosystem's
canonical runner: `go test ./...`, `cargo test`, the package.json test
script (via the lockfile's package manager), pytest, and gradle/maven.
Verdicts are PASS with counts where the output allows, FAIL
with the failing tests named, and NOT run when a runner or test script
is absent, which is never reported as green. `--coverage` adds the
percentage where the runner measures it natively (Go; pytest with
pytest-cov); a number is reported, never enforced. With
`--name <pattern>` narrows the run to matching tests — `-run` for Go,
`-k` for pytest, `--tests` for gradle, `-Dtest=` for maven, the pattern
after `--` for a JS test script, and a positional for cargo. A runner
that cannot express the pattern reports NOT filtered rather than
silently running everything, and zero matches is a pass saying
so. With `[test] policy = "block"` in config.toml, `todo close` and
`backlog close story` run the suite and refuse while it is red — or
while it cannot be verified at all, because unknown is never done.

#### `procoder status`

The state of play, computed fresh: current branch against the default,
dirty file count, the active sprint with its done/total and open
stories, open todo tasks, unlearned lessons, and index freshness. Every
line is a computed fact; anything that cannot be read says so with the
reason rather than defaulting to something comfortable. The same block
is injected at session start, inside a hard three-second budget that
never runs the gate, the suite, or any network tool.

#### `procoder run [--exec]`

How to run this project: the launch command(s) it declares — package.json
scripts, Makefile targets, a Go main, a Cargo bin, manage.py, docker
compose, a Procfile — each with the file and line that declared it,
most specific first. Procoder does not manage processes: a server is
long-running, and backgrounding and log capture belong to the shell that
owns it. `--exec` runs a single one-shot candidate (120s, stdin closed)
and refuses when there is a choice to make or the command looks like a
server. A repository with nothing to run says so and exits 0.

#### `procoder env [--sync]`

What changed in the project's environment since you last synced:
lockfile digests per ecosystem with the install command to run,
migrations added or removed, and keys declared in an `.env.example` that
the local `.env` lacks — key **names** only, never a value from either
file. `--sync` records the current tree as the new baseline, which is a
statement that you have installed and migrated. Report-only: drift is
judgment, never a block. Files git ignores are never surveyed.

#### `procoder security [--deep]`

Secrets over the changed files with gitleaks — always blocking, values
never echoed, rotation ordered. `--deep` adds semgrep SAST (ERROR blocks)
and osv-scanner dependency vulnerabilities (CVSS ≥ 7.0 blocks) over the
repository.

#### `procoder maintain`

Dead-code candidates from the index's precise tier, cyclomatic complexity
and function length from isolated linter runs. Nothing blocks; thresholds
are the repo's to set (`[maintain]` in config.toml).

#### `procoder docs [--external]`

Broken relative references and non-compiling Mermaid diagrams block; doc
drift, missing API doc comments, required docs, badges, README structure,
version-tracked pages, and command coverage report. `--external` adds
lychee link checking and GitHub Pages health.

#### `procoder ci [--runs]`

Workflow hygiene: actions pinned to mutable refs (report by default,
`[ci] pin_actions_policy = "block"` to block), missing per-job
`timeout-minutes`, missing concurrency cancellation, and pipelines without
tests.

#### `procoder infra`

Where the files exist: hadolint over Dockerfiles, `terraform fmt` /
`validate` / tflint over Terraform (a failing validate blocks),
kubeconform over Kubernetes manifests, `helm lint` over charts.

### Specs, plans, and tasks

The chain: a spec says what and why, a plan says how exactly, todos track
gated execution — and each link has its own quality controller.

#### `procoder todo <sub>`

The quality-gated task list under `.procoder/todo/` — one Markdown file
per task, with a real description, testable acceptance criteria, and an
evidence section.

- `add <title>` — prints the task file and its path; the agent writes it
  and replaces the placeholders before starting work.
- `list` / `show <id>` — every task (open first) / one task in full.
- `close <id>` — the quality controller. It refuses to close until every
  acceptance criterion is checked, the evidence section records what was
  run and what it proved, and the commit gate is clean — and it names
  exactly what is missing. Only a passing close moves `Status:` to closed.

#### `procoder spec <sub>`

Spec-first design under `.procoder/specs/` — the `/procoder:spec` skill
interviews the gaps closed; the binary judges completeness.

- `template <name>` — prints the spec shape (Problem, Users, In/Out of
  scope, Constraints, Interfaces, Data, Edge cases, Failure modes,
  Acceptance criteria, Open questions) for the agent to write.
- `list` — every spec in the repo.
- `check [name|all]` — the quality controller: blocks while any required
  section is missing or empty, while any `OPEN:` question is unresolved,
  and while acceptance criteria are not testable checkboxes. A complete
  spec whose `Status:` line still says `draft` earns a note to advance it
  to `complete` — a note, never a gap. A complete spec seeds the todo
  list — one task per criterion group.

#### `procoder plan <sub>`

Implementation plans under `.procoder/plans/`, written from an approved
spec for an engineer with zero context.

- `template <name>` — prints the plan shape (Goal, Architecture,
  Constraints, `## Task N:` blocks with Files, Interfaces, checkbox
  steps).
- `list` — every plan in the repo.
- `check [name|all]` — the quality controller: blocks on placeholders
  ("TBD", "handle edge cases", "similar to task N" — a plan is written,
  not promised), on empty sections, and on tasks without `Files:` or
  checkbox steps.

#### `procoder debt`

Harvests deliberate-simplification markers into a ledger. Convention: a
corner cut on purpose carries a comment with the configured marker
(default `debt:`, `[debt] marker` in config.toml) naming the ceiling and
the condition to revisit. Markers with no revisit trigger are flagged —
those are the ones that silently rot. Read-only, never blocking.

#### `procoder agents`

The universal agent layer: per-host rule files (Cursor, Windsurf, Cline,
Kilo Code, Roo Code, Kiro, Antigravity, Qoder, Copilot editors, Codex)
derived from the canonical `AGENTS.md`. Prints the content for anything
missing or drifted so the agent can write it; drift blocks the gate.
See [Every agent](portability.md) for the full host matrix.

#### `procoder lessons`

The self-learning loop's ledger (`.procoder/github/LESSONS.md`): every
finding that escaped the repository's gates and was caught downstream becomes a
lesson, and every lesson must carry the adaptation that closes its class —
a linter enabled, a line added to the pre-PR review rubric
(`.procoder/github/REVIEW.md`), a controller tightened, a pinning test.
Entries with no adaptation are flagged UNLEARNED and exit 1 — recorded is
not learned. An unreadable ledger exits 2.

#### `procoder copilot-leak [--since <dur>] [--quiet] [--from-copilot]`

What GitHub Copilot's auto-review caught that this repository's gates did
not. Findings are sanitised before anything is shown or sent — fenced and
indented code stripped, secrets redacted, absolute paths made relative — so
what leaves the machine is metadata about a failure, never the source that
failed. Nothing is published without an explicit yes on a terminal; with no
terminal to ask, it asks nothing and captures nothing.

A captured finding becomes a GitHub issue and an entry in
`.procoder/github/COPILOT-LEAKS.md`, deliberately not `LESSONS.md`: a raw
finding is not yet a lesson until a human names its class and its adaptation.
`--from-copilot` reads that ledger back, listing each entry as learned or
UNLEARNED, and exits 1 while any remain unclassified. `--quiet` reports the
count without asking. Without `gh`, or outside a GitHub repository, it says
what it could not check rather than reporting zero.

#### `procoder principles`

Prints the engineering principles each session starts with (a SessionStart
hook injects them): build-ladder first — reuse, stdlib, platform, then the
minimum code that works — the delegation discipline (independent work
fans out to parallel subagents under a clear contract, watched as it
lands, nothing merged unjudged), and ADHD/ASD-friendly formatting for
complex answers: a title and one-line summary, type-labeled problem
cards, decisions in their own numbered list, noise filtered, and short
single-topic answers left plain. A repo replaces them wholesale with
`.procoder/PRINCIPLES.md`.

### The code index

#### `procoder index <sub>`

- `build` — both tiers: universal-ctags (broad) + SCIP (precise) into
  `.procoder/index/` (gitignored), stamped with the commit.
- `find <symbol>` / `search <text>` / `outline <file>` — definitions,
  fuzzy lookup, a file's symbols in order.
- `refs <symbol>` — every reference, labeled precise (SCIP) or textual.
- `impls <symbol>` — what implements an interface or its methods.
  Precise tier only: implementation relationships exist nowhere else,
  so without SCIP the answer is "not built", never a textual guess.
- `callers <symbol>` / `graph` — the call graph and its JSON edge list.
- `unused` — dead-code candidates, exported API marked.
- `entrypoints` — mains and the exported surface.
- `impact` — the blast radius of the working-tree change.
- `stats` — what's indexed and staleness, said out loud.
- `rename <symbol> <new> [--at path:line]` — the cross-file rename as a
  reviewable unified diff, computed by the language's own engine (Go via
  gopls). Per P-CONTROL nothing is written: the agent reviews and applies
  the diff itself. Languages without an engine answer with the
  reference worksheet (`refs`) instead of a half-right rewrite; `--at`
  picks one definition when the name is defined more than once.

### Setup and plumbing

#### `procoder doctor`

Which tools this repository needs (by its file inventory), which are
installed, versions, and the install command for each gap.

#### `procoder init [--yes]`

Prints one install command per missing tool for this machine's package
managers; `--yes` executes them and re-surveys — an installer exiting 0 is
a claim, the tool resolving is the fact.

#### `procoder templates`

Prints the default content for any missing repo file Procoder reads:
the PR/commit/workflow templates under `.procoder/github/`, the docs and
security rules, the Mermaid theme.

#### `procoder scrub <file|->`

Checks text for AI-attribution lines; exit 1 when any are found.

#### `procoder hook post-tool-use`

The write hook's entry point — reads a PostToolUse payload on stdin and
answers with findings for the file just written. Wired by the plugin;
rarely invoked by hand.

#### `procoder version`

Prints the version.
