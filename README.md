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
| `/procoder-rot` | Find dead, stale, and deprecated code left behind. |  <!-- procoder: literal alone/deprecated-no-trigger the doctrine names this pattern, it is not an instance of it -->
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

`.procoder.toml` configures the check engine itself (rung severities,
thresholds, and exclusions):

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

[baseline]
file = ".procoder-baseline.json"
```

The values shown for `[thresholds]` and `[baseline]` are the defaults; every
key is optional. Arrays may span multiple lines. A line the config parser
cannot recognize is warned on stderr and skipped, never silently dropped.

`[rungs]` sets each rung's severity. `error` findings are reported as must-fix;
`warn` findings are reported as advisory. The active level modulates this: at
`pragmatic`, the judgment rungs (OBVIOUS, ALONE) are flagged without blocking
language, while SAFE and TRUE always demand a fix. The keys are the four rung
names verbatim, `true` included — it is a bare key, not a boolean.

`[exclude] paths` excludes whole paths (directory prefixes) from every check.
`[exclude] rules` is narrower and exact-path-only: each entry is
`path:rule-id`, and `path` must match a file exactly — no directory prefixes,
no globs. It silences one named rule (e.g. `obvious/complexity`) on one named
file, everywhere else that rule still applies. Because the match is exact
rather than a prefix, a rule-scoped exclusion cannot silently widen to cover
new files added later under the same directory — narrowing enforcement always
takes a second, deliberate edit per file.

### The literal marker

The third and narrowest suppression form is not in `.procoder.toml` at all: it
is a marker written on the line itself, for text that *describes* a violation
rather than committing one — a detection pattern, a doctrine paragraph, a test
fixture, a rule id quoted in config.

`<comment syntax> procoder: literal <rule-id>[, <rule-id>…] <reason>` <!-- procoder: literal alone/blanket-suppression the marker syntax written out, not a suppression -->

| Rule | Why |
|---|---|
| It must name its rules, comma-separated. There is no bare form and no wildcard. | An unnamed suppression is a rung-4 violation by this project's own doctrine; the bare form is reported as `alone/blanket-suppression`. |
| It must state a reason of at least two words. | A marker with no reason does not parse, silences nothing, and is reported as `alone/unexplained-suppression`. |
| The reason runs to end of line, so the marker is always last on its line. | That is what separates a *trailing* marker (this line only) from a *standalone* one (its own line and the next), which is the widest scope offered — there is no block or file form. |

The standalone form exists for lines that cannot carry a comment of their own:
YAML frontmatter, a markdown table row, a fenced example.

It can name any built-in rule, `safe/hardcoded-secret` included — a mechanism
that could not cover a credential could not cover the fixtures and doctrine
pages where the false positives actually are. What keeps it honest is that it
names the rule, states a reason, reaches at most two lines, and sits in the
diff right next to the value.

It can also name an external linter's rule id — `true/eslint:no-eval` — which
carries a colon of its own. A rule id it does not recognise is dropped from the
marker rather than silently honoured: the finding still reports, and the unknown
id is named on stderr, so a typo fails loudly instead of quietly widening what
the marker covers. See [`docs/known-limitations.md`](docs/known-limitations.md)
for what it does not cover.
### `.procoderignore`

A `.procoderignore` file may sit in **any** directory and excludes paths in that
directory and everything beneath it — so a large generated subtree is excluded
by one file next to it, instead of a central list that grows stale far from what
it describes.

```
# gen/.procoderignore
*.gen.ts        # any depth below gen/
/vendor/        # only gen/vendor/, not gen/pkg/vendor/
build/          # a directory of that name, at any depth
!keep.gen.ts    # ...except this one
```

The syntax is a **subset** of `.gitignore`, and this list is exhaustive —
anything not on it is not supported:

| Supported | Meaning |
|---|---|
| `# comment`, blank lines | Skipped. Leading and trailing whitespace is trimmed. |
| `name` | Matches `name` — and everything under it if it is a directory — at any depth below the ignore file. |
| `a/b.ts` | A slash anywhere anchors the pattern to the ignore file's own directory. |
| `/vendor` | A leading slash anchors it too. |
| `build/` | A trailing slash matches only a directory's contents, never a file of that name. |
| `*` | Any run of characters within one path segment. |
| `**` | Any run of characters across segments; `**/` also matches zero directories, so `b/**/*.ts` covers `b/x.ts`. |
| `!pattern` | Negation — puts a path back into the gate. |

**Not supported**, and treated as literal characters rather than silently doing
something else: `?`, character classes (`[a-z]`), and backslash escapes. A line
that cannot be compiled is dropped and matches nothing; a `.procoderignore` that
cannot be read, or is full of nonsense, ignores nothing and never fails a hook.

**Precedence**, in order:

1. `.procoder.toml`'s `[exclude] paths` is decided first and its verdict is
   final. A `!` in a `.procoderignore` cannot re-include what the root config
   excluded — the root config is the project-wide contract, and a subdirectory
   may narrow further but never contradict it.
2. Otherwise the last matching pattern wins, and a deeper `.procoderignore`'s
   patterns are applied after a shallower one's — so the deepest file decides,
   as in git.
3. Only ignore files in the path's own ancestor directories are ever read, so a
   `.procoderignore` cannot affect, or reach above, its own directory. A pattern
   naming a parent (`../`) is dropped.

`.procoderignore` is a **pure path filter**: there is no `path:rule-id` form.
Silencing one rule on one file stays a deliberate, exact-path edit in
`.procoder.toml`, where it is visible in one place and `verify
--unused-exclusions` can tell you when it has gone stale.

**`.gitignore` is deliberately not read.** A file being untracked says something
about version control, not about whether it should ship — and the PostToolUse
hook fires on files git has never seen. Inheriting those patterns would narrow
the gate by reusing a file written for another purpose, silently, which is the
one thing this project refuses to do. Ignoring a path from procoder takes a line
in a file whose name says so.

Because an ignore file narrows enforcement, `procoder check` reports what it
covered — one line on stderr per ignore file that actually skipped something:

```
procoder: 412 files skipped by gen/.procoderignore — not checked.
```

Per ignore file, not per file: the case this exists for is a large generated
subtree, and a line per file would bury the findings it was meant to make room
for. `check`, `baseline` and `verify` share the engine, so a file one of them
ignores is ignored by all three and the ratchet cannot disagree with itself.
Symlinks are not resolved: a path is matched exactly as it was written, so a
symlinked directory is judged by the name it was reached through.

## Statusline

procoder ships a statusline script that prints the active level (e.g.
`[PROCODER:STRICT]`), or nothing when procoder is off or inactive. One command
wires it up:

```bash
procoder statusline install
```

It writes the `statusLine` block into your Claude Code settings
(`$CLAUDE_CONFIG_DIR/settings.json`, or `~/.claude/settings.json`), resolving
the script path from where this copy of procoder actually lives and picking the
`.sh` or the `.ps1` for your platform. Everything else in that file is left as
it was, the previous version is copied to a timestamped `.backup-` file first,
and running it twice changes nothing the second time. A `statusLine` that is not
procoder's is reported, not replaced — pass `--force` if you mean to replace it.

```bash
procoder statusline status     # what is configured today
procoder statusline uninstall  # remove procoder's entry, leave everything else
```

If you would rather edit the file yourself — or the installer declined because
your install path contains characters a shell would interpret — the block it
writes is just:

```json
{
  "statusLine": {
    "type": "command",
    "command": "bash /path/to/procoder/hooks/procoder-statusline.sh"
  }
}
```

Substitute the real install path, quoted or escaped to suit your shell. On
Windows the equivalent is
`powershell -NoProfile -File C:\path\to\procoder\hooks\procoder-statusline.ps1`.

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
