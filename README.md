# procoder

[![CI](https://github.com/azrtydxb/procoder/actions/workflows/ci.yml/badge.svg)](https://github.com/azrtydxb/procoder/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/azrtydxb/procoder)](LICENSE)
[![Node](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fraw.githubusercontent.com%2Fazrtydxb%2Fprocoder%2Fmain%2Fpackage.json&query=%24.engines.node&label=node&color=brightgreen)](package.json)
[![Runtime dependencies](https://img.shields.io/badge/runtime%20dependencies-0-brightgreen)](package.json)

<!-- Each badge above reads its value from something that fails when the value
     changes: the CI badge from the `ci` workflow, the licence badge from the
     repository's own LICENSE, the node badge from `engines.node` in
     package.json, and the zero-dependency badge from tests/manifest.test.js,
     which asserts `pkg.dependencies === undefined`. No badge states a number
     nobody keeps honest. -->

**An AI writes the change in seconds. What it almost never does is delete what
the change replaced.** So the old function survives — still exported, still
imported somewhere, still wrong. Six weeks later somebody calls it, or patches
it instead of its replacement, or reads it and believes it.

procoder is a [Claude Code](https://claude.com/claude-code) plugin and CLI that
refuses to call a change done until that thing is gone.

That is its fourth rung, **ALONE**, and it has no equivalent in an ordinary
linter. eslint and ruff answer questions about the code in front of them; none
of them has a concept of *the thing this replaced*. `/procoder:rot` does: it
hunts superseded exports, orphaned branches, parallel implementations of one
job, and it checks reachability before it recommends deleting anything, so what
it cannot prove is reported as needing confirmation rather than deleted.

Three rungs sit in front of it, in cost order — **SAFE**, **TRUE**,
**OBVIOUS** — and all four are checked while the code is being written, not at
review time. The doctrine is injected at session start, so the model is
*following* the rungs before anything is checked, and a `PostToolUse` hook runs
the deterministic engine over every file the agent writes. A finding lands in
the same turn that produced it, when the change is still cheap to undo, instead
of in a pull request comment three days later.

Pointing a gate at an existing codebase usually produces thousands of findings
and gets uninstalled the same afternoon. `procoder baseline` records what is
already there as accepted, and `procoder verify` fails only on what is new: the
accepted set may shrink, never grow. Adopting procoder on a legacy repository
does not start with a cleanup.

It pairs with [`ponytail`](https://github.com/dietrichgebert/ponytail):
ponytail decides **what to write** (the minimal, YAGNI-first solution),
procoder decides **whether it may ship**.

**Documentation: <https://azrtydxb.github.io/procoder/>** — the doctrine,
per-host install steps, every command, the full configuration reference, and
what the tool is known to miss, linked from every page rather than buried.

## The ladder

Ponytail's ladder is *stop at the first rung that holds* — a search. procoder's
ladder is **every rung must hold before it ships** — a gate. Checked in this
order because the cost of getting it wrong descends.

| # | Rung | Question | Negotiable |
|---|------|----------|------------|
| 1 | **SAFE** | Does untrusted data reach a sink unvalidated? | No |
| 2 | **TRUE** | Are errors handled and edges covered, with one runnable check left behind? | No |
| 3 | **OBVIOUS** | Would the next reader get it in one pass? | Judgment |
| 4 | **ALONE** | Did you leave a twin behind? | Judgment |

Rung 4 is the rung nobody ships, and the reason procoder exists: **a change
isn't done until the thing it replaced is gone.**

*(This table is copied from `skills/procoder/SKILL.md`, the single source of
the doctrine. It's the one place duplication is accepted — a README that
sends the reader to another file to learn what the tool does has failed at
its job. `scripts/sync-rules.js` keeps the generated rule files listed in
"Other platforms" below in sync with the doctrine automatically; this README
is not one of them, so keep this table in step by hand when the doctrine's
ladder changes.)*

## Where it does not defer

procoder runs the project's own linter rather than replacing it: where eslint,
ruff, golangci-lint or clippy is configured, its findings replace procoder's
own shape rules, because the project's linter defines the project's shape.

Rung 1 is the exception. Security findings are never handed to an external
linter, because eslint and ruff do not check for SQL injection, shell
injection or disabled TLS verification by default, and a gate that silently
delegates its non-negotiable rung to a tool that does not check it is worse
than no gate at all.

The same reasoning is why procoder has **zero runtime dependencies** and a
lockfile with nothing in it. A tool whose first rung says a new dependency is a
new trust boundary does not get to add one for convenience.

## It gates itself

`tests/dogfood.test.js` runs procoder over the whole tracked tree, derived from
`git ls-files` so a file is inside the gate the day it lands, and the CI run
that gates a pull request is the same one. There is no hold-out list, and the
arithmetic is published instead of implied: of **201** tracked files the scan
reads **183** and skips **18** — 9 by `[exclude] paths` and 9 by two
`.procoderignore` files. Every skip is printed on every run with the pattern
that caused it, the same test asserts these three numbers against the scan
itself so they cannot drift from this paragraph, and every exclusion is
re-judged on every `verify`: one whose path is gone, that matches no file, or
whose files have all gone clean is reported, and fails under
`--unused-exclusions`. The whole-repository finding count — currently 0 — and
what each of the 18 buys is on
[what it misses](https://azrtydxb.github.io/procoder/limitations.html).

## Install

```bash
claude plugin marketplace add azrtydxb/procoder
claude plugin install procoder
```

(Replace the marketplace source with a local path if you're installing from a
clone: `claude plugin marketplace add ./procoder`.)

For a pre-commit hook, a CI job, or a terminal with no agent in it, the same
engine ships as a CLI:

```bash
npm install -g procoder
procoder check <paths...>
```

Cursor, Windsurf, Cline, Kiro, Qoder, opencode, openclaw, Codex, Copilot, pi
and any MCP host each read a rule file generated from the same doctrine.
[`docs/install.md`](docs/install.md) has exact, copy-pasteable steps for each,
and [Get started](https://azrtydxb.github.io/procoder/start.html) is the same
material on the site.

## Levels

| Level | Enforces |
|---|---|
| `off` | procoder is disabled entirely. |
| `pragmatic` | Rungs SAFE and TRUE enforced; OBVIOUS and ALONE flagged only, non-blocking. |
| `strict` (default) | All four rungs enforced on code touched this session. |
| `paranoid` | strict, plus a threat-model note on every new trust boundary, and ALONE applied to whole files rather than just the diff. |

Switch levels mid-session with `/procoder:level <level>`, or say "stop procoder" /
"normal mode" to deactivate. Deactivation is persisted, so it outlives the
session and survives restarts — re-enable with `/procoder:level strict` (or any other
level) when you want procoder back.

## Commands

| Command | Purpose |
|---|---|
| `/procoder:level` | Set the procoder intensity level: pragmatic, strict, or paranoid. |
| `/procoder:review` | Review the current diff against the four rungs. |
| `/procoder:audit` | Audit the whole repository, ranked by rung severity. |
| `/procoder:rot` | Find dead, stale, and superseded code left behind. |
| `/procoder:threat` | Map every trust boundary and what validates it. |
| `/procoder:deps` | Audit dependencies: vulnerable, abandoned, unpinned, unused. |
| `/procoder:debt` | Ledger of `procoder:` markers, flagging any without a removal trigger. |
| `/procoder:gain` | Measured progress: rot removed, boundaries hardened, baseline shrinkage. |
| `/procoder:guard` | Install procoder as a pre-commit hook and CI check. |
| `/procoder:statusline` | Install, inspect, or remove the statusline badge. |
| `/procoder:update` | Update procoder, and name what the update invalidates. |
| `/procoder:help` | Show procoder's rungs, levels, and commands. |

`/procoder:rot` and `/procoder:threat` are the two with no equivalent in
comparable tools: one hunts what previous changes left behind, the other maps
where untrusted data enters and what validates it. Each command is described in
full on [Commands](https://azrtydxb.github.io/procoder/commands.html).

See [`examples/`](examples/) for a worked before/after pair per rung — each
`before.*` trips its rung through `node bin/procoder.js check`, each `after.*`
is clean, and both are proved on every test run.

## Configuration

`.procoder.toml` configures the check engine — rung severities, shape
thresholds, and exclusions. Every key is optional; the values below are the
defaults:

```toml
[rungs]
safe = "error"
true = "error"
obvious = "warn"
alone = "warn"

[thresholds]
function_lines = 40
nesting_depth = 3
params = 4
complexity = 10

[exclude]
paths = ["vendor/", "generated/"]
rules = ["scripts/legacy-parser.js:obvious/complexity"]

[limits]
max_file_bytes = 2097152

[baseline]
file = ".procoder-baseline.json"
```

`[rungs]` sets each rung's severity — `error` findings are must-fix, `warn`
findings advisory — and the active level modulates it. The keys are the four
rung names verbatim, `true` included: it is a bare key, not a boolean. A line
the parser cannot recognize is warned about on stderr and skipped, never
silently dropped, and so is a value it cannot read exactly. A key written
**twice** has *both* values dropped and the key left unset, with a warning
naming the file and line — a duplicate says two things and TOML permits
neither, so keeping either one would be a guess. A repeated `[table]` header
and a dotted key colliding with a table are refused the same way.

`[exclude] paths` reports what it costs: one stderr line per pattern per run
with the count it held back, and the pattern named — and if you name an
excluded file on the command line, that file by name. `procoder verify` also
reports a configured path exclusion whose path no longer exists, and fails on
it under `--unused-exclusions`.

`[limits] max_file_bytes` is the largest file the engine will open — anything
above it is skipped and said so on stderr, never counted clean. 2 MB is a
measured ceiling, not a preference: past it the checks miss the hook's 2 s
budget. The key clamps **downward only**. A smaller value is honoured; a larger
one is refused with a warning naming file and line, and the built-in ceiling is
used instead — as are zero, a negative, and anything that is not a positive
number of bytes. Set it too low to admit any file and `procoder verify` says
how many files it could not check and exits **2**, "cannot verify", rather than
declaring a ratchet over a run that read nothing.

Two narrower instruments sit under it. A `.procoderignore` file may sit in
**any** directory and excludes paths beneath it, using a documented subset of
`.gitignore` syntax — `.gitignore` itself is deliberately not read. And a line
marker covers text that *describes* a violation rather than committing one:
<!-- procoder: literal alone/blanket-suppression the marker syntax written out, not a suppression -->
`<comment syntax> procoder: literal <rule-id>[, <rule-id>…] <reason>`
It must name its rules and give a reason, and it reaches the line it sits on —
or that line and the next, standing alone. For a finding reported at one line
but built at another, a marker on either of the two lines it names covers it.

Six environment variables tune the rest, `PROCODER_DEFAULT_LEVEL` and
`PROCODER_NO_HOOK` among them.
[Configuration](https://azrtydxb.github.io/procoder/configuration.html) is the
full reference for all of the above: every ignore pattern, every precedence
rule, the ratchet baseline, and every variable.

## MCP

For hosts that speak MCP but not Claude Code plugins, `procoder-mcp/server.js`
is a dependency-free JSON-RPC 2.0 (stdio) server exposing the same engine as
the hooks: `procoder_doctrine` (the rungs at a given level), `procoder_check`
(run the engine against a file), and `procoder_baseline` (read the ratchet
baseline). Point an `mcpServers` config at it:

```json
{
  "mcpServers": {
    "procoder": {
      "command": "node",
      "args": ["/path/to/procoder/procoder-mcp/server.js"]
    }
  }
}
```

See [`docs/install.md`](docs/install.md#mcp) for the full method list.

## Other platforms

The doctrine in `skills/procoder/SKILL.md` is the single source of truth.
`scripts/sync-rules.js` renders it into the rule file each non-Claude-Code
agent actually reads, and CI fails if any generated file drifts from the
source — so hand-editing one of these is not an option.

| File | Read by |
|---|---|
| `AGENTS.md` | Any agent following the emerging `AGENTS.md` convention. |
| `.cursor/rules/procoder.mdc` | Cursor |
| `.windsurf/rules/procoder.md` | Windsurf |
| `.clinerules/procoder.md` | Cline |
| `.kiro/steering/procoder.md` | Kiro |
| `.qoder/rules/procoder.md` | Qoder |
| `.agents/rules/procoder.md` | Generic `.agents`-convention tooling |
| `.openclaw/skills/procoder/SKILL.md` | OpenClaw |

`scripts/sync-rules.js` also ports each of the twelve `commands/*.toml` files to
`.opencode/command/<name>.md` and `.openclaw/commands/<name>.md` — opencode
reads `AGENTS.md` for the doctrine itself and these for the slash commands.

Run `npm run sync` after editing the doctrine or a command to regenerate all
of the above, or `npm run sync:check` to verify there's no drift.

## The documentation site

The site is served from [`docs/`](docs/) by GitHub Pages' own Jekyll build, so
the masthead, navigation and footer live in exactly one file each —
`docs/_layouts/page.html`, `docs/_includes/nav.html` and `docs/_data/nav.yml` —
rather than being copied into every page, which is the duplication rung 4
exists to catch. Nothing is added to this repository to make that work: no
package, no lockfile, no build step of your own to run. Edit a page under
`docs/` and the change is live on the next push to `main`.

The pages make no external request — no CDN, no web font, no analytics, no
badge image fetched from a third party — for the same reason the tool has no
runtime dependencies.
