# procoder

A harness that gives AI coders the tools and skills to work like a senior
developer.

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

1. **Security**
2. Best practices
3. Maintainability
4. Performance
5. Documentation
6. **Clean code / readability / pretty** ← shipped (v1: formatting)
7. CI / CD / CT
8. DevOps / IaaS / CaaS / infrastructure and deployment
9. **GitOps / GitHub / Git Actions** ← shipped (v1)

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
