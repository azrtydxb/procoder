# Changelog

Every release, in words a user can read. Newest first.

## 0.23.0 — 2026-08-19

Serena parity: the two capabilities that still needed the serena MCP
plugin now live in procoder, without giving up P-CONTROL.

- `procoder index rename <symbol> <new> [--at path:line]` — the
  cross-file rename as a reviewable unified diff, computed by the
  language's own engine (Go via gopls, which doctor now requires on Go
  repositories). Nothing is written: the agent reviews and applies the
  diff, then verifies with `index refs`. Languages without an engine
  answer honestly with the reference worksheet instead of a half-right
  rewrite; an ambiguous name lists every definition and asks for `--at`.
- `procoder lint --types` — the type-checker where the canonical linter
  does not compile the code: `tsc --noEmit` for TypeScript (grouped
  under each file's nearest tsconfig; without one the file is declared
  out of scope, never silently skipped) and pyright for Python. Go and
  Rust need no flag — golangci-lint and clippy already compile what
  they lint. Doctor requires tsc under a project tsconfig and pyright
  where Python is a real project (pyproject/requirements).
- The index skill now says when to reach past the index: a textual refs
  answer, same-named symbols, or interface implementations are the
  language server's job — use the host's native LSP tool and come back
  to the index for repo-wide sweeps.

## 0.22.1 — 2026-08-19

- The CLI help (`procoder` with no arguments) now lists every command
  alphabetically instead of grouped by workflow, so a command is found
  by name at a glance. Descriptions are unchanged.

## 0.22.0 — 2026-08-19

The language matrix: procoder now covers the popular languages end to end.

- Formatting adds Java (google-java-format), Kotlin (ktfmt), Swift
  (swiftformat, verified live), Ruby (rubocop autocorrect), Dart
  (dart format), and C# (csharpier) — joining Go, Python, the prettier
  family (JS/TS/JSON/CSS/HTML/Markdown/YAML), Rust, C/C++, and shell.
  Same contract everywhere: the project's config wins, the result is
  printed for the agent, unchecked is never clean.
- Lint adds cargo clippy (Rust, workspace-scoped, filtered to changed
  files), ktlint, swiftlint, rubocop, and checkstyle (google_checks
  baseline; a repo checkstyle.xml wins).
- The dependency scan enumerates the wider lockfile matrix: Maven and
  Gradle, .NET packages.lock.json, Swift Package.resolved, Elixir
  mix.lock, Dart pubspec.lock — one list shared with doctor, so the
  scanner requirement and the scan can never disagree. (Podfile.lock is
  deliberately excluded: osv-scanner has no extractor for it, verified.)
- The precise index tier adds rust-analyzer (Rust) and scip-java
  (Java/Kotlin/Scala builds); doctor recommends each only where the
  repository's files call for it.
- The broad index tier gains Swift and Dart: universal-ctags ships no
  parser for either, so procoder supplies regex-based definitions
  (top-level symbols, verified live) — every matrix language can now be
  found, searched, and outlined.
- Docs: an Influences page maps every idea absorbed from the superpowers
  and ponytail plugins to exactly where it lives in procoder; the
  quality-chain page now speaks its own name (spec-based development,
  design documents, quality gates) and carries a real verbatim
  spec-check refusal.
- Honesty note: Go/Python/JS/shell remain the continuously-tested paths
  (they gate this repo's own CI); Swift was verified against the live
  tool; the rest follow each tool's documented interface and fail
  honest — a wrong flag surfaces as NOT-checked, never as clean.

## 0.21.0 — 2026-08-19

The documentation overhaul, and the gates that keep it from rotting again.

- README rewritten from scratch: the whole product told value-first —
  the gate, the quality chain (spec → plan → todo), the self-learning
  loop, the nine domains, the code index, principles and debt, every
  agent — instead of a release-one story eleven versions stale.
- The docs site grew three pages (Getting started, The quality chain,
  Architecture) and a restructured landing; navigation now tells the
  product's story before its reference.
- The rot guards, because presence checks let this happen: a repo can
  declare its feature families (`## README must mention` in the docs
  rules) and a family the README stops telling blocks the gate;
  /procoder:pr gains the mandatory docs-impact question ("what does this
  change alter about what a reader must be told?") answered before any
  PR opens; and the review rubric carries the product-story line. The
  escape is recorded in the lessons ledger with all three adaptations.

## 0.20.0 — 2026-08-19

procoder now works with every AI coding agent, not just Claude Code.

- One canonical `AGENTS.md` carries the always-on contract; ten rule-file
  hosts (Cursor, Windsurf, Cline, Kilo Code, Roo Code, Kiro, Antigravity,
  Qoder, Copilot editors, Codex) get byte-pinned copies, and drift blocks
  the gate exactly like the PR-template mirror. `procoder agents` (and
  `/procoder:agents`) prints the content for anything missing or drifted.
- Plugin-tier adapters for Codex CLI (shares Claude's hooks file — the
  binary detects the host and answers in its JSON shape), GitHub Copilot
  CLI (own hook schema, bash+powershell), Gemini CLI/Antigravity
  (`contextFileName: AGENTS.md`), OpenCode (a thin JS shim plus generated
  command twins, parity pinned by test), Grok Build, Devin CLI, Qoder,
  pi, and Hermes. Adapter rule: adapters stay thin — logic lives in the
  binary, content in `AGENTS.md` and `commands/`.
- Host detection in the binary (`COPILOT_PLUGIN_DATA` → `PLUGIN_DATA` →
  `QODER_SESSION_ID` → Claude, with the VS Code Copilot path heuristic);
  `procoder principles --hook` emits each host's session-start shape.
- Manifest versions are pinned to the plugin version by the gate, and a
  root `hooks/hooks.json` is forbidden outright (Gemini would auto-load
  it with incompatible event names). Claude Code remains the tested
  reference host; the docs say so plainly.

## 0.19.0 — 2026-08-19

Catch first, learn on escape: downstream reviewers become the fallback,
not the net.

- The pre-PR self-review: `/procoder:pr` now dispatches a fresh-context
  reviewer over the branch diff against `.procoder/github/REVIEW.md`
  BEFORE the PR is opened; Critical/Important findings are fixed first.
  The default rubric is seeded from every class bot reviewers actually
  caught on this repo.
- The reflection loop: `/procoder:merge` treats an escaped finding as a
  bug in our gates — each one names the layer that should have caught it,
  that layer is adapted in the same PR, and the lesson lands in
  `.procoder/github/LESSONS.md`. `procoder lessons` flags entries with no
  adaptation as UNLEARNED (exit 1) — recorded is not learned. Our own
  ledger ships seeded with the eight real escapes to date: the PR #17/#18
  review findings, the CI mirror hang, and our own self-scan's fixture
  harvest.
- Go lint baseline: repositories without a golangci config get a curated
  default (standard set plus gosec, gocritic, errorlint, unparam,
  copyloopvar, nilerr) — the same pattern as the eslint baseline, and the
  repo's own config always wins.
- CI robustness: apt is repointed from the flaky Azure mirror to the
  canonical archive with fail-fast retries — a gate run once burned its
  whole timeout waiting on that mirror.
- Honesty fix from our own scanner: debt-marker test fixtures are now
  assembled at runtime so `procoder debt` on this repository reports a
  clean ledger instead of harvesting its own tests.

## 0.18.0 — 2026-08-19

Absorbed the best of the superpowers and ponytail plugins, so both can be
uninstalled.

- `procoder plan` and `/procoder:plan`: implementation plans under
  `.procoder/plans/` complete the spec → plan → todo chain. The quality
  controller blocks on placeholders ("TBD", "handle edge cases",
  "similar to task N"), empty sections, and tasks without `Files:` or
  checkbox steps — a plan is written, not promised.
- `procoder debt`: deliberate-simplification markers (`debt:` comments
  naming a ceiling and a revisit condition; marker configurable via
  `[debt]`) harvested into a ledger, with no-trigger markers flagged as
  rot.
- `procoder principles` plus a SessionStart hook: every session starts
  with the engineering principles (reuse → stdlib → platform → minimum
  code, root-cause bug fixing, deliberate corners marked as debt);
  `.procoder/PRINCIPLES.md` replaces them per repo.
- New skills: `/procoder:debug` (root cause before any fix, one
  hypothesis at a time, three strikes questions the architecture),
  `/procoder:tdd` (red before green, name the break each test catches,
  the mutation check), `/procoder:simplify` (the five-tag
  over-engineering review with an honest "Lean already. Ship." null
  result).
- Skill upgrades: `/procoder:spec` now classifies work as spike, bounded,
  or architectural with a one-way ratchet before interviewing;
  `/procoder:todo` defines what counts as evidence (fresh verification
  only, red-green proof for regression tests); `/procoder:merge` gains
  the review-receiving rules (verify claims before implementing, ask
  when unclear, facts instead of gratitude).

## 0.17.0 — 2026-08-19

Quality controllers for tasks and specs — done means verified.

- `procoder todo` and `/procoder:todo`: tasks live as Markdown files under
  `.procoder/todo/`, each with a real description, testable acceptance
  criteria, and an evidence section. `todo close` is the quality
  controller — it refuses to close a task until every criterion is
  checked, the evidence records what was run and what it proved, and the
  commit gate is clean, naming exactly what is missing.
- `procoder spec` and `/procoder:spec`: spec-first design under
  `.procoder/specs/`. The skill runs a gap-closing interview (problem,
  users, scope boundaries, constraints, interfaces, data, edge cases,
  failure modes, testable acceptance criteria, open questions);
  `spec check` blocks while sections are missing or empty, while any
  `OPEN:` question is unresolved, and while criteria are untestable.
  A complete spec seeds the todo list.
- The docs domain now requires CHANGELOG.md to carry an entry for the
  current version (blocking): a changelog that exists but skips the
  release being shipped is exactly how release notes go stale.

## 0.16.0 — 2026-08-19

The onboarding sweep, the comprehensive site, and a robustness batch.

- `procoder audit` and `/procoder:audit`: every domain's checks over the
  WHOLE tracked tree of a repository procoder has not governed before,
  aggregated into a triage-ordered scorecard. Its first run flagged our
  own pinned action SHAs as secrets — the false-positive flow
  (`gitleaks:allow` / `.gitleaksignore`, every allow a reviewed decision)
  is now part of the security rules.
- The docs site grew from one page to a real reference: the nine domains,
  every command, every configuration knob, and the workflow — and a new
  completeness check blocks a shipped command the documentation never
  mentions (usage text and the coverage list are pinned to each other by
  test).
- Robustness: CI runs once per change (push runs only on main), golangci
  caches are isolated per repository root (no more stale cross-worktree
  paths), the pr skill enforces ≤72-char titles, the merge skill deletes
  remote branches explicitly instead of trusting the flag's silent local
  failure, and the accepted Stdout.Write info line is excluded by config.

## 0.15.0 — 2026-08-19

Linters for all, without an asterisk — and the version tripwire now
guards every claims-bearing page.

- VersionSync generalizes from the README to a rules-driven list
  (## Version-tracked docs in .procoder/docs/RULES.md; default README.md
  and docs/index.md): the Pages site's index shipped eight releases
  stale because only the README was held to the version — the same
  prose-claims blind spot, now closed for every listed page. The site
  content itself is rewritten to the all-nine reality.

- Configless JavaScript now gets a procoder baseline: eslint's BUILT-IN
  core rules (no-undef, no-unused-vars, eqeqeq, no-var, …) via a
  generated temp flat config with common runtime globals — no npm
  packages installed, nothing written into the repo, and the project's
  own eslint config still always wins. Findings are labeled
  "(lint, procoder baseline)".
- TypeScript without a project config stays honestly out of scope: a TS
  parser is not built into eslint and installing one would be imposing.
- eslint v10 removed the unix formatter from core — both eslint paths now
  parse --format json, fixing config-carrying projects on v10 too.

## 0.14.1 — 2026-08-19

Morning review fixes, both dictated:

- hook.Run (complexity 25) and index Impact (25) refactored into named
  single-purpose helpers — both now under the threshold; the remaining
  switchboards (gate.Run 19 and friends) accepted as honest.
- Maintain thresholds are repo-overridable per D-OVERRIDE:
  `[maintain] gocyclo / funlen_lines / funlen_statements` in
  .procoder/config.toml, defaults 15/80/50.

## 0.14.0 — 2026-08-19

Domain 4, performance — and with it, all nine domains shipped.

- `/procoder:perf`: the measure-first discipline as a skill. Deterministic
  perf checks barely exist, so v1 teaches the real instruments (go test
  -bench/pprof/benchstat, cProfile/py-spy, node --cpu-prof) and the law:
  baseline, change, re-measure, report the delta with the command — a fix
  without a benchmark is a hope. Heavier tooling arrives when a real need
  shows.

## 0.13.0 — 2026-08-19

Domain 8, DevOps/IaaS/CaaS: each instrument only where its files exist.

- `procoder infra` and `/procoder:infra`: hadolint over Dockerfiles,
  `terraform fmt`/`validate`/tflint over *.tf directories (a failing
  validate BLOCKS — objectively broken; uninitialised dirs say NOT
  validated instead of failing on providers), kubeconform over Kubernetes
  manifests, `helm lint` over charts. Rides the gate and `procoder git`;
  a repo with no infrastructure pays nothing.
- doctor/init learn the five tools, each required only by inventory.

## 0.12.0 — 2026-08-19

Domain 7, CI/CD/CT: pipeline discipline as deterministic checks.

- `procoder ci` and `/procoder:ci`: mutable action refs (report by
  default, blocking via `[ci] pin_actions_policy = "block"`), missing
  per-job timeout-minutes, missing concurrency cancel-in-progress, and no
  tests anywhere. Rides `procoder git` and the gate too.
- Our own CI practices it: every action pinned to its commit SHA with the
  tag as a comment, and a concurrency group cancels stale runs.

## 0.11.0 — 2026-08-19

Domain 3, maintainability: a thin layer over the index and the linters.

- `procoder maintain` and `/procoder:maintain`: dead-code candidates from
  the precise index (exported API marked for judgment), cyclomatic
  complexity and function length from isolated linter runs with procoder's
  own thresholds (gocyclo 15, funlen 80/50, C901) — the repo's lint config
  is neither required nor touched. Everything reports; nothing blocks.

## 0.10.0 — 2026-08-19

Domain 1, security: the priority level, built on lint's rails and the index.

- Secrets (gitleaks): BLOCKING always — in the write hook the moment a
  secret lands in a file, in the gate over the changed set. The finding
  names rule and location, never the value, and orders a rotation.
- SAST (semgrep, community rules) and dependency vulnerabilities
  (osv-scanner): `procoder security --deep` and CI; ERROR severity and
  CVSS ≥ 7.0 block, the rest is judged.
- `/procoder:security` reviews from the index's entry points and call
  graph; rules live in .procoder/security/RULES.md.
- Missing scanners read as blocking NOT-checked — a security check that
  silently didn't run is worse than a red one.

## 0.9.0 — 2026-08-19

Domain 2, best practices: the canonical linter per ecosystem.

- `procoder lint` and `/procoder:lint`: golangci-lint (Go), ruff check
  (Python), shellcheck (shell), eslint (JS/TS, only where the project
  carries a config — procoder imposes no rules). The write hook lints the
  file just written in-turn; the gate lints the changed set.
- Report by default — lint is judgment, formatting was not; a repo opts
  into blocking with `[lint] policy = "block"` in .procoder/config.toml.
- Missing linters read as NOT checked, never clean; configless JS/TS is
  labeled out of scope.
- `/procoder:update`: update the plugin from the marketplace and verify
  the new version by direct invocation.

## 0.8.2 — 2026-08-19

- The README must carry the current version on its first screen — a
  blocking docs check, born from three releases shipping against a badge
  frozen at 0.7.0. Prose claims aren't file paths, so drift never fired;
  now a release without a reviewed README reds the gate. The README's
  domain list also caught up (documentation shipped, the index noted).
- The call graph dropped its noise: compiler-local temporaries and bare
  package descriptors are excluded from the edges (7,012 → 2,587 on this
  repo, all signal), `callers` shows each named call once with readable
  symbols (`io/ReadAll()`, not the SCIP provenance header).

## 0.8.1 — 2026-08-19

- The skills are back: command definitions moved from TOML with multiline
  strings — which the plugin loader silently drops — to Markdown with YAML
  frontmatter, the canonical format. Same nine commands, now actually
  registered.

## 0.8.0 — 2026-08-19

The code index (D-INDEX): the shared platform layer under the domains.

- `procoder index build|find|search|refs|outline|impact|stats` and the
  `/procoder:index` skill. Broad tier from universal-ctags (definitions,
  outlines, fuzzy search); precise tier from SCIP (scip-go and friends) for
  exact references — every answer says which tier produced it, and a stale
  index says so out loud.
- `index impact`: the blast radius of the working-tree change — which
  symbols it defines and which files reference them; the gate prints it
  and /procoder:pr makes the agent verify the named files.
- The security/maintainability surface, built now: `index callers` (the
  call graph from SCIP occurrences), `index unused` (dead-code
  candidates, exported API marked), `index entrypoints` (mains and the
  exported surface), and `index graph` (the machine-readable edge list
  future domains walk).
- The write hook keeps the broad tier current for each file written; the
  gate rebuilds a stale index at the finishing moment, covering editor
  edits and the precise tier the hook cannot reach.
- Tool resolution got honest: a probe rejects macOS's BSD ctags impostor,
  and `~/go/bin` / `~/.local/bin` count as installed.

## 0.7.1 — 2026-08-19

- The docs scan now asks git which Markdown belongs to the repository
  (tracked plus untracked-but-not-ignored) instead of walking every
  directory — gitignored scratch is no longer scanned.
- The PR-template mirror is enforced: drift between .github/ (the path
  GitHub reads) and the .procoder/github/ master now blocks the gate.
- The merge watcher got a protocol: calibrate against previous runs, poll
  per job in the foreground, report the first failure immediately with its
  log excerpt, poll dynamically — never a fire-and-forget monitor.
- Issue templates caught up with the reset: no more dropped "levels",
  Node-era fields, or renamed config paths.

## 0.7.0 — 2026-08-19

Domain 5, documentation: docs treated as a product.

- `procoder docs [--external]` and `/procoder:docs`: broken relative
  references and non-compiling Mermaid diagrams block; doc drift, missing API
  doc comments (Go/Python/TypeScript), required docs, badges, and README
  first-screen structure are reported; `--external` adds `lychee` link
  checking and GitHub Pages health.
- The write hook now checks Markdown references and diagrams in-turn, and
  reports which docs mention a code file the agent just changed.
- New repo-owned rules: `.procoder/docs/RULES.md` and the shared Mermaid
  theme `.procoder/docs/mermaid.json` (printed by `procoder templates`).
- Docs site: MkDocs Material built and deployed to GitHub Pages by CI.
- `doctor`/`init` learn `lychee`, `mmdc`, and `mkdocs`.

## 0.6.0 — 2026-08-18

Repo-overridable workflow rules (D-OVERRIDE begins here).

- `.procoder/github/WORKFLOW.md`: feature work in git worktrees, PR polling
  delegated to a watch-only background agent, full local+remote cleanup after
  a successful merge. The repo's file wins over the skills' defaults.
- Fixed `RepoRoot` to recognize a worktree's `.git` file — commands run
  inside a worktree no longer report against the parent checkout.

## 0.5.x — 2026-08-17

Domain 9, GitOps/GitHub, hardened by its own dogfood runs.

- The gate's git slice: conflict markers, junk files, oversized files
  (5 MB default), AI-attribution lines in commit messages (blocking — the
  work is the author's), commit subject shape, default-branch policy.
- `actionlint` on every workflow file written, findings in the same turn.
- PR and commit templates under `.procoder/github/`, `/procoder:pr` and
  `/procoder:merge` skills, `procoder scrub`.
- 0.5.1 fixed the gate exiting 0 over blocking findings; 0.5.2 fixed Windows
  test stubs and stopped prettier flagging the commit template's functional
  blank lines.

## 0.4.0 — 2026-08-17

- `procoder init [--yes]`: the binary computes the install plan per machine,
  the agent (or `--yes`) executes it, and the survey re-runs afterwards —
  an installer's exit 0 is a claim; the tool resolving is the fact.

## 0.1.0 – 0.3.0 — 2026-08-17

The Go rewrite and the plumbing proof (domain 6, formatting).

- One static binary per platform, committed in `dist/`, installed via the
  Claude Code marketplace; hooks and skills call a thin launcher.
- Formatting via each ecosystem's canonical tool (gofmt, ruff, prettier,
  rustfmt, clang-format, shfmt) with three honest verdicts: clean,
  unformatted (formatted bytes handed to the agent), unchecked (said out
  loud, never silent). The write hook hands the agent the formatted code and
  never touches the file (P-CONTROL).

Before 0.1.0 the project was a TypeScript analyzer engine; that history is in
git. The design reset that produced the current harness is recorded in the
design contract.
