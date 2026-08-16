# procoder

procoder is a [Claude Code](https://claude.com/claude-code) plugin that governs
whether AI-written code is allowed to ship. Every response is checked against
four rungs — **SAFE**, **TRUE**, **OBVIOUS**, **ALONE** — covering security at
trust boundaries, error handling and edge cases, readability, and whether the
code the change replaced was actually deleted. It pairs with the
[`ponytail`](https://github.com/dietrichgebert/ponytail) plugin: ponytail decides **what to write** (the minimal, YAGNI-first
solution); procoder decides **whether it may ship**.

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

## Install

```bash
claude plugin marketplace add azrtydxb/procoder
claude plugin install procoder
```

(Replace the marketplace source with a local path if you're installing from a
clone: `claude plugin marketplace add ./procoder`.)

## Levels

| Level | Enforces |
|---|---|
| `off` | procoder is disabled entirely. |
| `pragmatic` | Rungs SAFE and TRUE enforced; OBVIOUS and ALONE flagged only, non-blocking. |
| `strict` (default) | All four rungs enforced on code touched this session. |
| `paranoid` | strict, plus a threat-model note on every new trust boundary, and ALONE applied to whole files rather than just the diff. |

Switch levels mid-session with `/procoder <level>`, or say "stop procoder" /
"normal mode" to deactivate. Deactivation is persisted, so it outlives the
session and survives restarts — re-enable with `/procoder strict` (or any other
level) when you want procoder back.

## Commands

| Command | Purpose |
|---|---|
| `/procoder` | Set the procoder intensity level: pragmatic, strict, or paranoid. |
| `/procoder-review` | Review the current diff against the four rungs. |
| `/procoder-audit` | Audit the whole repository, ranked by rung severity. |
| `/procoder-rot` | Find dead, stale, and deprecated code left behind. |
| `/procoder-threat` | Map every trust boundary and what validates it. |
| `/procoder-deps` | Audit dependencies: vulnerable, abandoned, unpinned, unused. |
| `/procoder-debt` | Ledger of `procoder:` markers, flagging any without a removal trigger. |
| `/procoder-gain` | Measured progress: rot removed, boundaries hardened, baseline shrinkage. |
| `/procoder-guard` | Install procoder as a pre-commit hook and CI check. |
| `/procoder-help` | Show procoder's rungs, levels, and commands. |

`/procoder-rot` and `/procoder-threat` have no equivalent in comparable tools:
one hunts what previous changes left behind, the other maps where untrusted
data enters and what validates it.

See [`examples/`](examples/) for a worked before/after pair per rung, run
through the real check engine.

## Examples and install docs

[`examples/`](examples/) has one before/after pair per rung — each `before.ts`
trips its rung through `node bin/procoder.js check`, each `after.ts` is clean.

[`docs/install.md`](docs/install.md) has exact, copy-pasteable install steps
for every supported host: Claude Code, Cursor, Windsurf, Cline, Kiro, Qoder,
opencode, openclaw, Codex/Copilot, pi, MCP, and CLI/CI-only.

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

## Configuration

Environment variables:

| Variable | Effect |
|---|---|
| `PROCODER_DEFAULT_LEVEL` | Overrides the default level (`strict`) for new sessions. Must be one of `off`, `pragmatic`, `strict`, `paranoid`. |
| `PROCODER_NO_HOOK` | Set to `1` to disable all procoder hooks (activation, level tracking, subagent propagation) without uninstalling the plugin. |
| `CLAUDE_CONFIG_DIR` | Overrides where procoder persists the active level (`<dir>/.procoder-active`). Defaults to `~/.claude`. |
| `PROCODER_HOST` | Selects the hook wire protocol for non-Claude-Code hosts. One of `codex`, `copilot`, `qoder`. Unset (or any other value) uses the native Claude Code protocol. `codex` is also auto-detected via `CODEX_HOME`. |

`.procoder.toml` configures the check engine itself (level, thresholds, and
exclusions):

```toml
[rungs]
safe = "error"
true_ = "error"
obvious = "warn"
alone = "warn"

[exclude]
paths = ["vendor/", "generated/"]
rules = ["scripts/legacy-parser.js:obvious/complexity"]
```

`[rungs]` sets each rung's severity. `error` findings are reported as must-fix;
`warn` findings are reported as advisory. The active level modulates this: at
`pragmatic`, the judgment rungs (OBVIOUS, ALONE) are flagged without blocking
language, while SAFE and TRUE always demand a fix. The key is `true_` rather
than `true`, since bare `true` is a TOML boolean.

`[exclude] paths` excludes whole paths (directory prefixes) from every check.
`[exclude] rules` is narrower and exact-path-only: each entry is
`path:rule-id`, and `path` must match a file exactly — no directory prefixes,
no globs. It silences one named rule (e.g. `obvious/complexity`) on one named
file, everywhere else that rule still applies. Because the match is exact
rather than a prefix, a rule-scoped exclusion cannot silently widen to cover
new files added later under the same directory — narrowing enforcement always
takes a second, deliberate edit per file.

## Statusline

procoder ships a statusline script that prints the active level (e.g.
`[PROCODER:STRICT]`), or nothing when procoder is off or inactive. Add it to
your Claude Code settings, pointing at wherever the plugin is installed on
your machine:

```json
{
  "statusLine": {
    "type": "command",
    "command": "bash /path/to/procoder/hooks/procoder-statusline.sh"
  }
}
```

A PowerShell equivalent, `procoder-statusline.ps1`, is included for Windows —
use `powershell -File /path/to/procoder/hooks/procoder-statusline.ps1`.

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

`scripts/sync-rules.js` also ports each of the ten `commands/*.toml` files to
`.opencode/command/<name>.md` and `.openclaw/commands/<name>.md` — opencode
reads `AGENTS.md` for the doctrine itself and these for the slash commands.

Run `npm run sync` after editing the doctrine or a command to regenerate all
of the above, or `npm run sync:check` to verify there's no drift.
