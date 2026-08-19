# Command reference

Every procoder command, what it computes, and what blocks. The binary only
ever computes and reports — the agent (or you) acts on the results
(P-CONTROL).

## Onboarding

### `procoder audit`

The onboarding sweep for a repository procoder has not governed before:
every domain's checks over the WHOLE tracked tree — formatting verdicts,
hygiene, secrets, lint — aggregated into one scorecard with a triage
order. Exit 1 while the repository would fail the gate; the
`/procoder:audit` skill drives the fixing.

## Everyday commands

### `procoder check [paths...]`

The commit gate. Over the changed files (or the given paths): formatting
(unformatted and unchecked both fail), git hygiene (conflict markers, junk
and caches, oversized files, AI-attribution lines — all blocking), workflow
lint, CI hygiene, infrastructure hygiene, lint findings, secrets (blocking),
docs checks, and the change's blast radius from the index. Exit 1 on any
blocking finding.

### `procoder git`

The pre-finish status: branch vs default, changed-file count, template
presence and registration, and every hygiene finding — the same rules as
`check`, shared through one code path so they can never disagree.

### `procoder format <files...>`

Prints each file's formatted result (gofmt, ruff, prettier, rustfmt,
clang-format, shfmt — the project's config always wins) so it can be
reviewed and written. Never touches the file.

### `procoder lint [paths...]`

The canonical linter per ecosystem: golangci-lint (Go), ruff check
(Python), shellcheck (shell), eslint (JS/TS — configless plain JavaScript
gets the built-in-rules procoder baseline; configless TypeScript is out of
scope), cargo clippy (Rust), ktlint (Kotlin), swiftlint (Swift), rubocop
(Ruby), and checkstyle (Java, google_checks baseline). Go repositories without a golangci config get procoder's curated
baseline (standard set plus gosec, gocritic, errorlint, unparam,
copyloopvar, nilerr) — the repo's own golangci config always wins,
whichever of `.golangci.yml`/`.yaml`/`.toml`/`.json` it uses. Report
by default; `[lint] policy = "block"` makes findings block.

### `procoder security [--deep]`

Secrets over the changed files with gitleaks — always blocking, values
never echoed, rotation ordered. `--deep` adds semgrep SAST (ERROR blocks)
and osv-scanner dependency vulnerabilities (CVSS ≥ 7.0 blocks) over the
repository.

### `procoder maintain`

Dead-code candidates from the index's precise tier, cyclomatic complexity
and function length from isolated linter runs. Nothing blocks; thresholds
are the repo's to set (`[maintain]` in config.toml).

### `procoder docs [--external]`

Broken relative references and non-compiling Mermaid diagrams block; doc
drift, missing API doc comments, required docs, badges, README structure,
version-tracked pages, and command coverage report. `--external` adds
lychee link checking and GitHub Pages health.

### `procoder ci`

Workflow hygiene: actions pinned to mutable refs (report by default,
`[ci] pin_actions_policy = "block"` to block), missing per-job
`timeout-minutes`, missing concurrency cancellation, and pipelines without
tests.

### `procoder infra`

Where the files exist: hadolint over Dockerfiles, `terraform fmt` /
`validate` / tflint over Terraform (a failing validate blocks),
kubeconform over Kubernetes manifests, `helm lint` over charts.

## Specs, plans, and tasks

The chain: a spec says what and why, a plan says how exactly, todos track
gated execution — and each link has its own quality controller.

### `procoder todo <sub>`

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

### `procoder spec <sub>`

Spec-first design under `.procoder/specs/` — the `/procoder:spec` skill
interviews the gaps closed; the binary judges completeness.

- `template <name>` — prints the spec shape (Problem, Users, In/Out of
  scope, Constraints, Interfaces, Data, Edge cases, Failure modes,
  Acceptance criteria, Open questions) for the agent to write.
- `list` — every spec in the repo.
- `check [name|all]` — the quality controller: blocks while any required
  section is missing or empty, while any `OPEN:` question is unresolved,
  and while acceptance criteria are not testable checkboxes. A complete
  spec seeds the todo list — one task per criterion group.

### `procoder plan <sub>`

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

### `procoder debt`

Harvests deliberate-simplification markers into a ledger. Convention: a
corner cut on purpose carries a comment with the configured marker
(default `debt:`, `[debt] marker` in config.toml) naming the ceiling and
the condition to revisit. Markers with no revisit trigger are flagged —
those are the ones that silently rot. Read-only, never blocking.

### `procoder agents`

The universal agent layer: per-host rule files (Cursor, Windsurf, Cline,
Kilo Code, Roo Code, Kiro, Antigravity, Qoder, Copilot editors, Codex)
derived from the canonical `AGENTS.md`. Prints the content for anything
missing or drifted so the agent can write it; drift blocks the gate.
See [Every agent](portability.md) for the full host matrix.

### `procoder lessons`

The self-learning loop's ledger (`.procoder/github/LESSONS.md`): every
finding that escaped our own gates and was caught downstream becomes a
lesson, and every lesson must carry the adaptation that closes its class —
a linter enabled, a line added to the pre-PR review rubric
(`.procoder/github/REVIEW.md`), a controller tightened, a pinning test.
Entries with no adaptation are flagged UNLEARNED and exit 1 — recorded is
not learned. An unreadable ledger exits 2.

### `procoder principles`

Prints the engineering principles each session starts with (a SessionStart
hook injects them): build-ladder first — reuse, stdlib, platform, then the
minimum code that works. A repo replaces them wholesale with
`.procoder/PRINCIPLES.md`.

## The code index

### `procoder index <sub>`

- `build` — both tiers: universal-ctags (broad) + SCIP (precise) into
  `.procoder/index/` (gitignored), stamped with the commit.
- `find <symbol>` / `search <text>` / `outline <file>` — definitions,
  fuzzy lookup, a file's symbols in order.
- `refs <symbol>` — every reference, labeled precise (SCIP) or textual.
- `callers <symbol>` / `graph` — the call graph and its JSON edge list.
- `unused` — dead-code candidates, exported API marked.
- `entrypoints` — mains and the exported surface.
- `impact` — the blast radius of the working-tree change.
- `stats` — what's indexed and staleness, said out loud.

## Setup and plumbing

### `procoder doctor`

Which tools this repository needs (by its file inventory), which are
installed, versions, and the install command for each gap.

### `procoder init [--yes]`

Prints one install command per missing tool for this machine's package
managers; `--yes` executes them and re-surveys — an installer exiting 0 is
a claim, the tool resolving is the fact.

### `procoder templates`

Prints the default content for any missing repo file procoder reads:
the PR/commit/workflow templates under `.procoder/github/`, the docs and
security rules, the Mermaid theme.

### `procoder scrub <file|->`

Checks text for AI-attribution lines; exit 1 when any are found.

### `procoder hook post-tool-use`

The write hook's entry point — reads a PostToolUse payload on stdin and
answers with findings for the file just written. Wired by the plugin;
rarely invoked by hand.

### `procoder version`

Prints the version.
