# procoder

**Make your AI coder work like a senior developer** — all nine domains of
senior work, from security scans to merge discipline, delivered as tools the
agent runs itself and hooks it cannot skip. The agent stays in control;
nothing ever touches your code behind its back.

![CI](https://github.com/azrtydxb/procoder/actions/workflows/ci.yml/badge.svg)
![Version](https://img.shields.io/badge/version-0.20.0-0e5563)
![License](https://img.shields.io/badge/license-Apache--2.0-blue)

## Quick start

Install from the Claude Code plugin marketplace, then let it set itself up:

```
/plugin marketplace add azrtydxb/procoder
/plugin install procoder
/procoder:init          # installs the tools this repository needs
```

Every write is now checked in-turn, and the skills (`/procoder:check`,
`/procoder:git`, `/procoder:pr`, `/procoder:merge`, `/procoder:docs`, …) give
the agent a senior developer's routine.

```mermaid
flowchart LR
    A[agent writes code] --> H[hook fires]
    H --> B[procoder binary checks]
    B -->|findings + fixed content| A2[agent reviews and implements]
    A2 --> A
```

## How it works

It acts in two modes, and both matter:

- **Self-serve** — the agent gets tools and skills it runs itself: check a
  file, fetch a formatted result, ask which tooling is missing.
- **Forced** — certain tasks run at fixed points of the coding session's
  lifecycle, whether or not the agent chooses to. Hooks run them.

One principle governs every tool: **the agent always stays in control.** Tools
compute results and hand them to the agent; nothing modifies code, files, or
state behind the agent's back. The write hook does not format your file — it
gives the agent the formatted code, and the agent reviews and writes it.

## Domains

The harness is organised into nine domains of senior-developer work, delivered
level by level:

1. **Security** ← shipped (v1)
2. **Best practices** ← shipped (v1: lint)
3. **Maintainability** ← shipped (v1)
4. **Performance** ← shipped (v1: skills)
5. **Documentation** ← shipped (v1)
6. **Clean code / readability / pretty** ← shipped (v1: formatting)
7. **CI / CD / CT** ← shipped (v1)
8. **DevOps / IaaS / CaaS** ← shipped (v1)
9. **GitOps / GitHub / Git Actions** ← shipped (v1)

Beneath the domains sits the **code index** (shipped) — the shared platform
layer described below that the agent and future domains query.

Domain 6 ships first because its formatter slice is deterministic — one
canonical tool per ecosystem, zero false positives, the fix is the tool's own
output — so every failure while proving the harness is a harness failure,
never a judgment call. Security is level 1 and the priority; it is built on
the proven plumbing.

## What works today: formatting

| language                               | formatter      | project config honoured                                                                                           |
| -------------------------------------- | -------------- | ----------------------------------------------------------------------------------------------------------------- |
| Go                                     | `gofmt`        | gofmt is the config                                                                                               |
| Python                                 | `ruff format`  | `pyproject.toml` / `ruff.toml`                                                                                    |
| JS / TS / JSON / CSS / Markdown / YAML | `prettier`     | `.prettierrc*`, `package.json`                                                                                    |
| Rust                                   | `rustfmt`      | `rustfmt.toml`                                                                                                    |
| C / C++                                | `clang-format` | `.clang-format` — **required**; without it the file is out of scope, because procoder imposes no style of its own |
| Shell                                  | `shfmt`        | `.editorconfig`                                                                                                   |

Three honest verdicts, never collapsed into each other:

- **clean** — the tool ran and the file matches its output
- **unformatted** — the tool ran; its output is handed to the agent to review
  and write
- **unchecked** — the tool is missing or failed; said out loud, counted by the
  gate as failing, **never** reported as clean

### The forced task

`PostToolUse` on every `Write`/`Edit`: the file's formatter runs, and if the
file is unformatted the agent receives the cleanly formatted code in the same
turn — with a note that the file itself was not touched.

### The skills

- `/procoder:format [files]` — print the formatted result for review
- `/procoder:check [paths]` — the commit gate: formatting plus git hygiene;
  unchecked fails like unformatted; out-of-scope files are counted out loud
- `/procoder:doctor` — which tools this repository needs, which are installed,
  and the install command for each gap
- `/procoder:init` — install the missing tools, every command visible
- `/procoder:git` — the pre-finish status: branch vs default, hygiene,
  message checks, workflow lint, template state
- `/procoder:pr` — gate, real-diff summary, template filled, attribution
  scrubbed, then `gh pr create` with everything shown first
- `/procoder:merge` — every check green and every review thread (Copilot
  included) fixed or answered before the merge — never over a red check

## GitOps / GitHub (domain 9, v1)

The gate's git slice, all deterministic:

- **conflict markers** left in a changed file — blocks, names file and line
- **junk files staged** (`.DS_Store`, `*.orig`, `*.rej`, …) — blocks
- **oversized files** (default 5 MB, `.procoder/config.toml`) — blocks
- **AI attribution in commit messages** (`Co-Authored-By: Claude`,
  "generated with", the anthropic noreply) — **blocks**; the work is the
  author's
- **commit subject shape** (≤72 chars, blank line before body) — reported
- **working on the default branch** — reported; a config switch makes it block
- **GitHub Actions lint** — `actionlint` runs on every workflow file you
  write, findings return in the same turn

Everything procoder owns lives in **`.procoder/`**: `config.toml`, and the
two Markdown templates under `.procoder/github/` —
`PULL_REQUEST_TEMPLATE.md` (filled by `/procoder:pr`) and
`COMMIT_TEMPLATE.md` (registered as `git config commit.template`). Plain
files, made to be edited. `procoder templates` prints defaults for anything
missing; the agent writes them.

## Documentation (domain 5, v1)

Documentation is treated as a product: correct, presentable, delivered.

- **broken relative references** and **Mermaid diagrams that do not compile**
  — block, in the gate and in-turn on every Markdown write
- **external links** — verified by `procoder docs --external` and CI with
  `lychee` (retries + cache); never skipped, never in the write hook
- **doc drift** — change a file a doc mentions and the doc-map reports both
  sides so the agent verifies the prose is still true
- **API doc comments** — exported Go / public Python / exported TypeScript
  symbols without docs are reported
- **README structure and badges** — the first screen must sell the project:
  USP one-liner, badges, quick start; `CHANGELOG.md` is required
- **GitHub Pages** — the docs site is built by CI (MkDocs Material) and
  `procoder docs --external` confirms Pages serves the latest build

Rules live in `.procoder/docs/RULES.md` (with the shared Mermaid theme in
`.procoder/docs/mermaid.json`) — plain files, repo-owned, they win over every
built-in default.

## The code index

The shared layer under the domains, and the agent's fast map of the code:

```
procoder index build            # both tiers: universal-ctags + SCIP
procoder index find <symbol>    # where it's defined
procoder index search <text>    # fuzzy symbol search
procoder index refs <symbol>    # every reference — precise or textual, labeled
procoder index outline <file>   # a file's symbols in order
procoder index impact           # the blast radius of your working-tree change
procoder index stats            # what's indexed, and staleness said out loud
```

The index lives in `.procoder/index/` (gitignored — derived, per-machine),
the write hook keeps it current, and future domains (best practices,
security) read the same files.

## Implementation

One Go binary (`cmd/procoder`), no runtime dependencies, cross-compiled per
platform into `dist/` and committed with the plugin — install from the Claude
Code marketplace and it works: no npm, no network at hook time, air-gapped
included. `hooks/launcher.sh` (and `.cmd` on Windows) picks the binary by
OS/architecture; hooks and skills only ever name the launcher.

```
go test ./...        # the test suite
go build ./cmd/procoder
```

The full design contract — the nine domains, the lifecycle, what each domain
owes before it ships — lives in the design document and supersedes anything
here that drifts from it.
