# The nine domains

procoder organises senior-developer work into nine domains. Each follows
the same architecture: the binary computes findings, the write hook hands
them to the agent in the same turn, the gate carries them at commit time,
a skill packages the workflow, and every rule is repo-overridable
(D-OVERRIDE). This page is the detailed reference: what each domain
checks, with what, what blocks, and where its knobs live.

Reading the tables: **blocks** fails the gate and CI; **reports** informs
the agent's judgment; **NOT-checked** means the tool was missing or
failed — counted as failing, never as clean.

## 1. Security

Runs in the write hook (changed file), the gate (changed set), and
`procoder security --deep` (whole repository).

| Check                                 | Tool                                                   | Verdict                                                                                |
| ------------------------------------- | ------------------------------------------------------ | -------------------------------------------------------------------------------------- |
| Secrets in any written/changed file   | gitleaks (with a legacy-CLI fallback for old installs) | **blocks** — names rule and location, never the value, and orders removal AND rotation |
| SAST over the repository (`--deep`)   | semgrep, community rulesets                            | ERROR severity **blocks**; WARNING reports                                             |
| Dependency vulnerabilities (`--deep`) | osv-scanner over explicitly named lockfiles            | CVSS ≥ 7.0 **blocks**; below reports                                                   |
| Scanner missing or output unreadable  | —                                                      | **blocks** as NOT-checked                                                              |

Details worth knowing: manifests are enumerated explicitly (osv's own
walker trusts git metadata and comes back empty in worktrees); a
`package.json` that declares dependencies with no lockfile is an explicit
unscannable gap; false positives are handled by `gitleaks:allow` trailing
comments or `.gitleaksignore`, each a reviewed decision — the flow is in
`.procoder/security/RULES.md`, which the repo owns.

## 2. Best practices (lint)

The canonical linter per ecosystem, under the project's own config —
procoder imposes nothing where the repo has spoken.

| Ecosystem | Tool          | Baseline when the repo has no config                                                                                  |
| --------- | ------------- | --------------------------------------------------------------------------------------------------------------------- |
| Go        | golangci-lint | curated set: standard + gosec, gocritic, errorlint, unparam, copyloopvar, nilerr                                      |
| Python    | ruff check    | ruff's defaults                                                                                                       |
| Shell     | shellcheck    | shellcheck's defaults                                                                                                 |
| JS/TS     | eslint        | plain JS gets eslint's built-in core rules; configless TypeScript is out of scope (a parser would have to be imposed) |
| Rust      | cargo clippy  | clippy's defaults (needs a Cargo workspace; findings filtered to the changed files)                                   |
| Kotlin    | ktlint        | ktlint's defaults                                                                                                     |
| Swift     | swiftlint     | swiftlint's defaults                                                                                                  |
| Ruby      | rubocop       | rubocop's defaults                                                                                                    |
| Java      | checkstyle    | the bundled google_checks; a repo `checkstyle.xml` wins                                                               |

Report by default; `[lint] policy = "block"` in config.toml makes
findings block. Lint is judgment where formatting was not — the findings
arrive in-turn on every write, and the agent is expected to fix what is
real and say why for what is not.

## 3. Maintainability

`procoder maintain` — informed judgment, never blocking.

| Check                 | Source                                              | Notes                                                                              |
| --------------------- | --------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Dead-code candidates  | the index's precise (SCIP) tier                     | exported API is marked — a public surface is legitimately unreferenced from inside |
| Cyclomatic complexity | golangci (gocyclo) / ruff (mccabe), isolated config | threshold `[maintain] gocyclo`, default 15                                         |
| Function length       | funlen                                              | `[maintain] funlen_lines` / `funlen_statements`, defaults 80/50                    |

Deliberate corner-cuts are the sibling discipline: mark them with the
`debt:` comment convention (marker configurable via `[debt]`) naming the
ceiling and revisit condition; `procoder debt` harvests the ledger and
flags no-trigger entries as rot.

## 4. Performance

`/procoder:perf` encodes measure-first: baseline before touching,
profile before guessing, re-measure after, report the delta with the
command that produced it. A fix without a benchmark is a hope. No
binary checks here yet by design — performance claims without
measurement infrastructure would violate the honesty contract.

## 5. Documentation

Documentation is a product: correct, presentable, delivered, and —
since it has burned us — **complete**.

| Check                                                                                                                        | Verdict                  |
| ---------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| Broken relative references, non-compiling Mermaid diagrams                                                                   | **blocks** (hook + gate) |
| Version-tracked pages missing the current version; changelog without an entry for the release                                | **blocks**               |
| A shipped command the docs never mention                                                                                     | **blocks**               |
| A declared feature family the README's narrative stops telling (`## README must mention`, whole-word, badges/links stripped) | **blocks**               |
| Doc drift (a doc mentions a file you changed), missing API doc comments, badges, README first screen                         | reports                  |
| External links (lychee), Pages serving the latest build                                                                      | `--external` and CI      |

Rules live in `.procoder/docs/RULES.md`; this site is built and deployed
by the harness's own CI job.

## 6. Clean code (formatting)

Every write is checked against the ecosystem's canonical formatter —
gofmt, ruff format, prettier (JS/TS/JSON/CSS/HTML/Markdown/YAML),
rustfmt, clang-format (config required — procoder has no style opinion
of its own), shfmt, google-java-format, ktfmt (Kotlin), swiftformat,
rubocop (Ruby), dart format, and csharpier (C#). Three verdicts, never
collapsed: **clean**, **unformatted** (the agent receives the formatted
result in-turn and writes it itself), **unchecked** (tool missing or
failed — fails the gate). The file is never touched behind the agent's
back; that is P-CONTROL's original case.

## 7. CI/CD/CT

`procoder ci` and the same checks inside the gate:

| Check                                                            | Verdict                                                         |
| ---------------------------------------------------------------- | --------------------------------------------------------------- |
| Actions pinned to mutable refs (a tag can be silently repointed) | reports; `[ci] pin_actions_policy = "block"` to block           |
| Missing per-job `timeout-minutes`                                | reports — a hung job otherwise burns the whole runner allowance |
| Missing concurrency cancellation                                 | reports                                                         |
| Pipelines with no test step                                      | reports                                                         |
| Workflow files unreadable                                        | **blocks** as NOT-checked                                       |

actionlint runs on every workflow file the agent writes, in-turn.

## 8. DevOps / IaaS / CaaS

`procoder infra` — inventory-driven: each tool runs only where its files
exist, so a repo without infrastructure pays nothing.

| Files                | Tool                                                                                                                                                                           | Verdict |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- |
| Dockerfiles          | hadolint                                                                                                                                                                       | reports |
| Terraform            | terraform fmt (reports) · terraform validate (**blocks** when initialised; says NOT-validated when `.terraform` is absent rather than failing on providers) · tflint (reports) |
| Kubernetes manifests | kubeconform                                                                                                                                                                    | reports |
| Helm charts          | helm lint                                                                                                                                                                      | reports |

## 9. GitOps / GitHub

The finishing discipline — most of it rides the gate:

| Check                                                                             | Verdict                                           |
| --------------------------------------------------------------------------------- | ------------------------------------------------- |
| Conflict markers in changed files                                                 | **blocks**, names file and line                   |
| Junk/caches staged (.DS_Store, \*.orig, node_modules, …)                          | **blocks**                                        |
| Oversized files (`[git] max_file_mb`, default 5)                                  | **blocks**                                        |
| AI-attribution lines in commits (`procoder scrub` for drafts)                     | **blocks** — the work is the author's             |
| PR-template mirror drift (`.github/` vs the `.procoder/github/` master)           | **blocks**                                        |
| Agent-layer drift (rule copies vs `AGENTS.md`, manifest versions)                 | **blocks**                                        |
| Commit subject shape (≤72, blank line before body); working on the default branch | reports (`[git] default_branch_policy` can block) |

Around the checks, the skills encode the workflow: worktree per feature,
`/procoder:pr` (docs-impact question, pre-PR self-review, scrubbed
template), `/procoder:merge` (watch-only polling, every review thread
answered, the reflection step for anything that escaped, then merge and
full cleanup).

## Beneath them: the code index

Two tiers — universal-ctags for breadth, SCIP for precision — with
eleven queries from `find` to the call `graph`, kept current by the hook,
consumed by the agent and the domains alike (maintainability's dead-code
sweep and the gate's impact lines both read it).

The language matrix, stated honestly:

- **Broad tier** (find/search/outline/textual refs/impact): everything
  universal-ctags parses — 160+ languages including C/C++/C#, Java,
  Kotlin, Ruby, Rust, PHP — plus procoder-supplied regex parsers for the
  two it lacks, Swift and Dart (top-level symbols, approximate by
  nature).
- **Precise tier** (exact refs/callers/graph): where a SCIP indexer
  exists and is wired — Go (scip-go), TypeScript (scip-typescript),
  Python (scip-python), Rust (rust-analyzer), and Java/Kotlin/Scala
  builds (scip-java). One indexer runs per repository (the first layout
  match); everything else answers textually and says so in the output —
  a textual ref is labeled, never passed off as precise.

## Above them: the quality chain

The domains judge code that exists; the [quality chain](quality-chain.md)
governs whether the right thing gets built at all — spec, plan, todo,
and the lessons loop, each with its own refusing controller.
