# The ten domains

**Reference.** Procoder organises senior-developer work into ten domains. Each follows
the same architecture: the binary computes findings, the write hook hands
them to the agent in the same turn, the gate carries them at commit time,
a skill packages the workflow, and every rule is repo-overridable
(D-OVERRIDE). This page is the detailed reference: what each domain
checks, with what, what blocks, and where its knobs live. For the same
checks grouped by _when_ they run — session start, every write, the
commit gate, CI — see [when each check runs](lifecycle.md).

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
Procoder imposes nothing where the repo has spoken — but it never checks
nothing. A linter that could not run is BLOCKING, in every domain: a green
gate has to mean the code was checked, not that the machine was empty.
`[lint] policy` governs whether a linter's findings block; whether the
linter ran at all is not a matter of policy.

The commit gate runs SAST and blocks on findings in the files the commit
carries. The scan itself is the same whole-tree scan `security --deep`
runs; the narrowing is applied to its findings, not to its targets —
naming files explicitly makes semgrep scan ones its own default selection
skips, which would block a developer on a finding CI never reports.

Known dependency vulnerabilities are checked at the gate too, but only
when the commit touches a manifest — the scan answers about the manifests,
so running it on a commit that edits a comment would report the same
vulnerabilities forever at nearly a second each time.

Every manifest in the repository is scanned when one changes, including
the ones beneath the root — a monorepo keeps one per package — and all of
them rather than only the one that changed, since a lockfile edit moves
the versions the others resolve against. Vendored copies and installed
packages are skipped: their manifests describe code nobody here can
change.

It costs seconds rather than milliseconds: semgrep's time goes on loading
rules, which is fixed, so a one-line file is barely cheaper than the whole
tree. It is there because a commit is not a keystroke and a finding caught
now never leaves the machine. A finding in a file the commit did not touch
does not block it; CI still reports everything.

C# and Dart are formatted and have no linter yet. They say so — `[lint]
policy` governs whether that blocks, the same as every other lint
finding, because there is no `procoder init` that fixes a language with
no linter at all. A tool procoder ships that is merely not installed
still blocks regardless of policy; see D-7 in
`.procoder/specs/no-silent-green.md`.

| Ecosystem | Tool           | Baseline when the repo has no config                                                                                                                          |
| --------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go        | golangci-lint  | curated set: standard + gosec, gocritic, errorlint, unparam, copyloopvar, nilerr                                                                              |
| Python    | ruff check     | ruff's defaults                                                                                                                                               |
| Shell     | shellcheck     | shellcheck's defaults                                                                                                                                         |
| JS/TS     | eslint         | plain JS gets eslint's built-in core rules; TypeScript with no project config gets Procoder's baseline through typescript-eslint (recommended set)            |
| Rust      | cargo clippy   | clippy's defaults (needs a Cargo workspace; findings filtered to the changed files)                                                                           |
| Kotlin    | ktlint         | ktlint's defaults                                                                                                                                             |
| Swift     | swiftlint      | swiftlint's defaults                                                                                                                                          |
| Ruby      | rubocop        | rubocop's defaults                                                                                                                                            |
| Java      | checkstyle     | the bundled google_checks; a repo `checkstyle.xml` wins                                                                                                       |
| PHP       | phpstan, phpcs | Procoder's curated phpstan baseline (level 5) when the repo configures neither; a repo `phpstan.neon` or `phpcs.xml` wins, and both run when both are present |
| C/C++     | clang-tidy     | Procoder's curated set — the analyser, bugprone and cert families; a repo `.clang-tidy` wins. Style is clang-format's job, not this one                       |

Report by default; `[lint] policy = "block"` in config.toml makes
findings block. Lint is judgment where formatting was not — the findings
arrive in-turn on every write, and the agent is expected to fix what is
real and say why for what is not.

`lint --types` closes the compile gap: Go and Rust arrive compiled
(their linters build what they lint), TypeScript and Python do not — so
the flag adds `tsc --noEmit` (under the project's own tsconfig) and
pyright. The agent reaches for it after refactors and renames, where
type fallout is exactly what the linters cannot see.

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

Complexity is Go and Python only — gocyclo rides golangci and mccabe
rides ruff, and no other ecosystem has a linter Procoder can isolate
the metric out of. Length and complexity are reported separately even
when they land on the same function: golangci keeps only the first
issue per line by default, and since a long function is usually a
branchy one, that default hid every length finding until 0.32.6. The dead-code sweep is limited to the index's precise
tier, so it answers for the languages a SCIP indexer covers and stays
silent elsewhere rather than guessing.

Dependencies age too, and `procoder deps` is the freshness half of the
same judgment — report-only, never blocking:

| Check                        | Tool                                                                   | Scope                                                       |
| ---------------------------- | ---------------------------------------------------------------------- | ----------------------------------------------------------- |
| Outdated direct dependencies | `go list -u -m`, `npm outdated`, cargo-outdated, pip — where installed | the ecosystems whose manifests exist; capped and summarized |
| Licenses                     | go-licenses                                                            | **Go only** — every other ecosystem answers NOT checked     |
| An optional tool missing     | —                                                                      | information, not failure; a tool that errored is a failure  |

NOT checked is reserved for a license surface that exists and was not
read. A repository whose manifest declares no third-party dependencies
has nothing to check, and says so — otherwise the reader learns to skim
the line in the repositories where it means something. Where Procoder
cannot tell (a manifest it cannot parse, or a Python project whose
dependencies live in `requirements.txt`, a `Pipfile`, or a `setup.py`
that computes them at runtime) it says NOT checked rather than guess
"none".

Nothing here decides for you: a major version behind is a fact, whether
to take it is a judgment with context Procoder does not have.

## 4. Performance

`/procoder:perf` encodes measure-first: baseline before touching,
profile before guessing, re-measure after, report the delta with the
command that produced it. A fix without a benchmark is a hope.

`procoder bench` is the measurement infrastructure that discipline used
to lack. It runs the repository's benchmarks (`go test -bench . -benchmem`)
and compares each against the committed baseline in
`.procoder/bench/baseline.txt`: ns/op and B/op with a percentage delta,
regressions beyond `[bench] threshold` (default 10) marked and exiting 1,
new and vanished benchmarks listed as such. `--save` records a new
baseline — explicit, because a baseline is a decision, not a side effect.

**Go only in this version**, and the output says so: other ecosystems
answer NOT run rather than letting the scope look wider than it is.
Results are single-run and machine-local; a baseline recorded on a
different GOOS/GOARCH still compares, with a warning attached. Numbers
arrive with the conditions that produced them.

## 5. Documentation

Documentation is a product: correct, presentable, delivered, and
**complete** — completeness has its own blocking checks, because
presence checks alone let documentation rot silently.

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

Decisions are documentation too, and they rot differently: prose can be
corrected, but a decision rewritten after the fact loses the reason it
was taken. `procoder adr` keeps them under `.procoder/adr/` as numbered,
immutable records — Context, Decision, Consequences, and a date — where
a changed mind writes a new record that supersedes the old one rather
than editing it. `adr check` refuses hollow records, unknown statuses,
duplicated numbers, and supersede references pointing at nothing; the
audit sweep carries those findings.

## 6. Clean code (formatting)

Every write is checked against the ecosystem's canonical formatter —
gofmt, ruff format, prettier (JS/TS/JSON/CSS/HTML/Markdown/YAML),
rustfmt, clang-format, shfmt, google-java-format, ktfmt (Kotlin), swiftformat,
rubocop (Ruby), dart format, csharpier (C#), and prettier with
`@prettier/plugin-php` for PHP. clang-format needs no project config: a
repo `.clang-format` wins, and without one Procoder names a fallback style
on the command line rather than skipping the file. A tool that cannot run —
prettier without the PHP plugin, say — is **unchecked**, which fails the
gate, never out of scope, which passes it. Three verdicts, never
collapsed: **clean**, **unformatted** (the agent receives the formatted
result in-turn and writes it itself), **unchecked** (tool missing or
failed — fails the gate). The file is never touched behind the agent's
back; that is P-CONTROL's original case.

## 7. Testing

`procoder test` runs the repository's actual suite — not a proxy for it,
and not a claim about it. Each detected ecosystem's canonical runner
runs and reports separately:

| Ecosystem | Runner                                                             | Coverage               |
| --------- | ------------------------------------------------------------------ | ---------------------- |
| Go        | `go test ./...`                                                    | native (`-cover`)      |
| Rust      | `cargo test`                                                       | not measured           |
| JS/TS     | the package.json `test` script, via the lockfile's package manager | not measured           |
| Python    | pytest (where a pytest config or a tests directory exists)         | native with pytest-cov |
| Java      | `./gradlew test` or `mvn -q test`, where the build files exist     | not measured           |

Three verdicts, and the third is the point: **PASS** with counts where
the output allows, **FAIL** with the failing tests named, and **NOT run**
when no runner or test script is present. NOT run is never green — a
repository with no suite is told it has no suite, not congratulated.
Exit 0 when everything passed, 1 when anything failed, 2 when nothing
could run at all.

`--coverage` reports the percentage where the runner measures it
natively. It is reported and never enforced: a threshold turns coverage
into a number to farm, and Procoder has no opinion worth blocking on
about which lines matter.

The suite reaches the rest of the chain through one knob. With
`[test] policy = "block"` in config.toml, `todo close` and
`backlog close story` run `procoder test` and refuse while it is red —
or while it cannot be verified at all, because unknown is never done.
Left at the default, the verdict informs and nothing refuses.

## 8. CI/CD/CT

`procoder ci` and the same checks inside the gate:

| Check                                                            | Verdict                                                         |
| ---------------------------------------------------------------- | --------------------------------------------------------------- |
| Actions pinned to mutable refs (a tag can be silently repointed) | reports; `[ci] pin_actions_policy = "block"` to block           |
| Missing per-job `timeout-minutes`                                | reports — a hung job otherwise burns the whole runner allowance |
| Missing concurrency cancellation                                 | reports                                                         |
| Pipelines with no test step                                      | reports                                                         |
| Workflow files unreadable                                        | **blocks** as NOT-checked                                       |

actionlint runs on every workflow file the agent writes, in-turn.

## 9. DevOps / IaaS / CaaS

`procoder infra` — inventory-driven: each tool runs only where its files
exist, so a repo without infrastructure pays nothing.

| Files                | Tool                                                                                                                                                                           | Verdict |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- |
| Dockerfiles          | hadolint                                                                                                                                                                       | reports |
| Terraform            | terraform fmt (reports) · terraform validate (**blocks** when initialised; says NOT-validated when `.terraform` is absent rather than failing on providers) · tflint (reports) |
| Kubernetes manifests | kubeconform                                                                                                                                                                    | reports |
| Helm charts          | helm lint                                                                                                                                                                      | reports |

## 10. GitOps / GitHub

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

The attribution check reads a named list of machine authors — Claude,
Codex, Copilot, Cursor, Devin, Gemini, aider, the robot emoji — and says
in the finding which one it matched. A `Co-Authored-By:` naming a person
is left alone; the header long predates AI coders and pair programming,
carried patches and squashes all use it correctly.

The attribution line is the one most likely to come back: the host adds
it, so amending the commit clears the finding without changing what
produced it. [The trailer your host
adds](portability.md#the-trailer-your-host-adds) lists the per-host
setting that stops it at the source.

Around the checks, the skills encode the workflow: a worktree per
feature (a git practice the skills prescribe — Procoder creates and
removes none of them itself), `/procoder:pr` (defer to an existing PR,
docs-impact question, pre-PR self-review, scrubbed template), `/procoder:merge` (watch-only
polling, every review thread answered, the reflection step for anything
that escaped, then merge and full cleanup).

Tagging is the last step and has its own controller. `procoder release`
verifies in one pass that every file in `[release] files` carries the
version, that CHANGELOG.md has the matching entry, that the tree is
clean including untracked files, that the gate is clean, and that the
suite is green under `[test] policy` — every failure listed together
rather than one per attempt. On success it prints the `git tag` command
and stops: the tag is the human's to run, as P-CONTROL requires. Without
`[release] files` the version-sync leg says out loud that it verified
nothing.

## Beneath them: the code index

Two tiers — universal-ctags for breadth, SCIP for precision — with
thirteen queries from `find` to the call `graph`, kept current by the hook,
consumed by the agent and the domains alike (maintainability's dead-code
sweep and the gate's impact lines both read it).

The language matrix:

- **Broad tier** (find/search/outline/textual refs/impact): everything
  universal-ctags parses — 160+ languages including C/C++/C#, Java,
  Kotlin, Ruby, Rust, PHP — plus Procoder-supplied regex parsers for the
  two it lacks, Swift and Dart (top-level symbols, approximate by
  nature).
- **Precise tier** (exact refs/impls/callers/graph): where a SCIP
  indexer exists and is wired — Go (scip-go), TypeScript
  (scip-typescript), Python (scip-python), Rust (rust-analyzer), and
  Java/Kotlin/Scala builds (scip-java). A polyglot repository runs
  every indexer its layout calls for and the results merge into one
  index; an ecosystem whose indexer is missing or failing stays
  textual and the build says so per indexer — a textual ref is
  labeled, never passed off as precise.
- **Rename** (`index rename`): the one write-shaped operation, and it
  still writes nothing — the language's own engine computes the
  cross-file rename (Go via gopls) and Procoder prints it as a unified
  diff for the agent to review and apply. A language without an engine
  gets the reference worksheet, not a half-right rewrite.

## Above them: the quality chain

The domains judge code that exists; the [quality chain](quality-chain.md)
governs whether the right thing gets built at all — spec, plan, the
backlog's milestones/epics/stories worked in sprints, the standalone
todo list, and the lessons loop, each with its own refusing controller.
