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
**OBVIOUS** — and two behind it that need the whole change in view: **FAST**
(does it stay cheap at production size?) and **MEANT** (is this what was asked
for, and only that?). All six are checked while the code is being written, not
at review time. The doctrine is injected at session start, so the model is
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
ladder is **every rung must hold before it ships** — a gate. Rungs 1–4 come in
cost order; 5 and 6 come last because neither can be answered from the line in
front of you.

| # | Rung | Question | Negotiable |
|---|------|----------|------------|
| 1 | **SAFE** | Does untrusted data reach a sink unvalidated? | No |
| 2 | **TRUE** | Are errors handled and edges covered, with one runnable check left behind? | No |
| 3 | **OBVIOUS** | Would the next reader get it in one pass? | Judgment |
| 4 | **ALONE** | Did you leave a twin behind? | Judgment |
| 5 | **FAST** | Does it stay cheap at the size production arrives at? | Judgment |
| 6 | **MEANT** | Is this what was asked for, and only that? | Judgment |

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
arithmetic is published instead of implied: of **211** tracked files the scan
reads **193** and skips **18** — 9 by `[exclude] paths` and 9 by two
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
procoder init --baseline      # a starter config, today's findings accepted
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
| `pragmatic` | Rungs SAFE and TRUE enforced; OBVIOUS, ALONE, FAST and MEANT flagged only, non-blocking — every rung whose `[rungs]` severity is `warn`. |
| `strict` (default) | All six rungs enforced on code touched this session. |
| `paranoid` | strict, plus a threat-model note on every new trust boundary, and ALONE applied to whole files rather than just the diff. |

Switch levels mid-session with `/procoder:level <level>`, or say "stop procoder" /
"normal mode" to deactivate. Deactivation is persisted, so it outlives the
session and survives restarts — re-enable with `/procoder:level strict` (or any other
level) when you want procoder back.

## Commands

| Command | Purpose |
|---|---|
| `/procoder:level` | Set the procoder intensity level: pragmatic, strict, or paranoid. |
| `/procoder:review` | Review the current diff against the six rungs. |
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

See [`examples/`](examples/) for a worked before/after pair for each of rungs
1–4 — there is no `fast/` or `meant/` pair, because the engine has no rule to
write one against. Each
`before.*` trips its rung through `node bin/procoder.js check`, each `after.*`
is clean, and both are proved on every test run.

## The CLI

The slash commands need a session; a pre-commit hook and a CI job do not have
one. The same engine answers to six subcommands:

| Command | Purpose |
|---|---|
| `procoder init [--baseline]` | Write a starter `.procoder.toml`. Nothing is ever overwritten — a config that already exists is left exactly as it is. `--baseline` also records today's findings as accepted, which is what makes an existing repository green on its first run rather than red by four thousand. |
| `procoder check [paths...]` | Report findings that are not in the baseline; exit 1 if any of them blocks at the active level. |
| `procoder baseline <paths...>` | Record every current finding as accepted. |
| `procoder verify <paths...>` | Exit 1 if a finding present today is not in the baseline — the CI ratchet. Exit 2 is *cannot verify*, never "you added findings". |
| `procoder rot <paths...>` | Rung 4 over the tree: index every export in the scan and report the ones nothing else mentions. |
| `procoder statusline <install\|uninstall\|status>` | Wire procoder's level badge into Claude Code's `statusLine`, or take it out again. `--append` keeps a statusLine you already have and prints the badge after it; `--force` replaces one that is not procoder's. |

`check --format text|json|sarif` decides who reads the output. `json` is
procoder's own versioned shape — `version`, `tool`, `level`, `summary`,
`findings[]`, `skipped[]` — so a consumer can tell an old document from a new
one. `sarif` is SARIF 2.1.0 for GitHub code scanning and anything else that
speaks it, and it carries `partialFingerprints.procoderFingerprint`: the same
fingerprint the ratchet uses, so a finding that moved down the file is not a new
alert to triage again. Findings go to stdout and every skip notice stays on
stderr, so the document stays parseable when you pipe it. Both formats name the
files the run skipped — JSON in a `skipped` array, SARIF as
`invocations[0].toolExecutionNotifications` with `executionSuccessful` false
when a file that was in scope could not be read — so an unread file cannot
arrive at a dashboard looking like a clean one. `--format` belongs to `check`;
typed at anything else it exits 2 rather than being ignored.

`check --since <ref>` checks what git says changed: `git diff --name-only
--diff-filter=ACMRT <ref>...HEAD`, plus anything uncommitted and anything
untracked. Renames, copies and type changes are included at their new path;
deletions are not, having nothing left to read. Paths given alongside narrow it
to the intersection, so `--since main src/` is "what changed under src/".
`--aging` and `--unused-exclusions` still apply, judged over what this run read.
A git failure exits 2 and names the command that failed — the CI template used
to do this in shell with `|| true`, where a first push made git fail, produced
no files, and passed green having checked nothing. Zero changed files says
`no files changed since <ref>` and exits 0; silence and a clean scan of a
hundred files must not look the same.

`--jobs <n>` sets how many worker processes the scan splits across. The default
is one per usable core — `os.availableParallelism()`, so a CPU quota is seen and
a one-core container does not fork eight workers — capped at 8, and **1 — this
process, no workers — for a run of fewer than 250 files**, where forking costs
more than it saves. A value above 8 is refused with a warning naming the value
and the ceiling; zero, negative and non-numeric are refused the same way and the
default is used. The report is identical either way: slices are contiguous,
reassembled in input order, every file runs at the same 2,000 ms budget, and a
worker that dies — or hangs past a derived slice bound — has its slice rescanned
here rather than dropped. `--jobs 1` is the way to take the pool out of the
picture entirely.

`--no-ignore` checks files a `.procoderignore` covers anyway — it answers "why
is this file not being checked?". It deliberately does not reach `[exclude]
paths` in `.procoder.toml`, which is the project-wide contract and carries the
built-in `node_modules/` defaults; every file that key holds back is reported
by count, or by name if you named that file on the command line.

`verify --unused-exclusions` also fails the build when an exclusion is holding
nothing back — a `[exclude] rules` entry that suppressed nothing this run, a
`[exclude] paths` entry whose path is gone or covers only clean files, or a
`.procoderignore` pattern in the same state. All three are reported under plain
`verify` and fail only under the flag, and the tree-wide judgments are made only
when the run's targets covered the whole repository.

`verify --aging <days>` names accepted debt that has outlived its welcome, with
each entry's date, path and rule, and exits 1. A baseline entry is a suppression
and a suppression with no end is rot, so this is the removal trigger the file
itself cannot carry. Without the flag, age never fails a run.

`rot` reports two tiers and exits 0 for both. A name nothing else mentions is a
deletion; a name mentioned outside its file only inside a string is *needs
confirmation*, because routing tables, DI containers and reflection all look
like that and an index of bare words cannot tell them apart. Files a published
package points at (`bin`, `main`, `module`, `exports` in package.json) and
conventional entry points (`index.*`, `lib.rs`, `mod.rs`, `__init__.py`) are
left out, since their callers are outside the scan. Test fixtures and example
files are exported-and-unmentioned by design and will show up: exclude them
under `[exclude] paths`, or read past them. Nothing is deleted, and a failing
build on a guess about deletion is how a tool gets switched off.

## Every rule the engine can report

Forty-four rule ids, and this is all of them — the set that line markers and
`[exclude] rules` entries are checked against. Rungs 5 (FAST) and
6 (MEANT) appear nowhere below because **no deterministic check produces a
finding at either**: they are doctrine the model reads, not gates CI runs.

Rung 1, SAFE — untrusted data reaching a sink, and credentials at rest:

| Rule id | Reports |
|---|---|
| safe/sql-injection | SQL built by interpolation, concatenation, `format!` or `Sprintf` reaching a query, cursor or statement. |
| safe/shell-injection | A shell command built by interpolation or concatenation, or a shell invoked with `-c` and an interpolated string. |
| safe/xss-sink | A raw HTML sink — `innerHTML`, `outerHTML`, `dangerouslySetInnerHTML` and their kin. |
| safe/dynamic-eval | Dynamic code evaluation: `eval`, `exec`, `Function(…)`, `new Function`. |
| safe/unsafe-deserialize | Deserialization of untrusted bytes — `pickle.loads`, `yaml.load`, Java native deserialization. |
| safe/xxe-risk | An XML parser created without external entities disabled. |
| safe/hardcoded-secret | A credential literal in source, or a credential-named identifier assigned a literal. |
| safe/redaction-marker | A redaction marker written **into** a file, meaning a real credential was overwritten with a placeholder. |
| safe/secret-in-log | A credential interpolated into a log call. |
| safe/pii-in-log | Personal data interpolated into a log call. |
| safe/tls-disabled | Certificate or hostname verification switched off. |
| safe/weak-hash | MD5 or SHA-1 used where a secure hash is expected. |
| safe/weak-random | A non-cryptographic RNG used for a token, key, nonce, salt or session id. |
| safe/unsafe-block | A Rust `unsafe` block with no `SAFETY:` comment. |
| safe/missing-lockfile | An ecosystem manifest with no lockfile committed beside it. |
| safe/manifest-not-locked | A dependency in `package.json` that the lockfile has never heard of — hand-edited rather than installed. |
| safe/floating-version | A dependency range that does not resolve to a version you audited. |

Rung 2, TRUE — errors handled, edges covered:

| Rule id | Reports |
|---|---|
| true/swallowed-error | An exception caught and silently discarded. |
| true/bare-except | A bare `except:`, which catches `SystemExit` and `KeyboardInterrupt` too. |
| true/ignored-error | An error assigned to `_` and dropped. |
| true/printstacktrace | An exception printed to stderr instead of handled. |
| true/missing-timeout | An outbound HTTP call or client with no timeout — Python `requests`, Go's empty `http.Client{}` and package-level helpers. |
| true/unclosed-resource | A resource opened with no visible close. |
| true/mutable-default | A mutable default argument, shared across calls. |
| true/panic-in-library | A `panic` in library code, which crashes the caller. |
| true/unwrap-in-library | `unwrap`/`expect` in library code, same reason. |
| true/budget-exhausted | The 2s per-file budget ran out before some stage ran — the file is only partly checked, and says so. |
| true/findings-suppressed | More than 20 findings on one line; the overflow is counted rather than dropped. |
| true/eslint, true/ruff, true/golangci-lint, true/clippy | A finding from the project's own linter that carried no rule id of its own. The tool's own ids arrive as `true/<tool>:<id>` and are checked on shape, since those namespaces are the tool's to define. |

Rung 3, OBVIOUS — thresholds from `[thresholds]`:

| Rule id | Reports |
|---|---|
| obvious/function-too-long | A function longer than `function_lines` (default 40). |
| obvious/nesting-depth | Nesting deeper than `nesting_depth` (default 3). |
| obvious/too-many-params | More parameters than `params` (default 4). |
| obvious/complexity | Branch count over `complexity` (default 10). |
| obvious/nested-ternary | A ternary inside a ternary. |

Rung 4, ALONE — what a change left behind:

| Rule id | Reports |
|---|---|
| alone/dead-export | An exported name nothing else in the scan mentions. Reported by `procoder rot`. |
| alone/commented-code | Commented-out code kept beside its replacement. |
| alone/debug-leftover | A debugging statement nobody removed. |
| alone/orphan-todo | A to-do note with no owner and no ticket id. | <!-- procoder: literal alone/orphan-todo the rule described, not an instance -->
| alone/deprecated-no-trigger | A deprecation mark with no removal condition. | <!-- procoder: literal alone/deprecated-no-trigger the rule described, not an instance -->
| alone/blanket-suppression | A suppression naming no rule, or disabling a whole file. |
| alone/unexplained-suppression | A suppression naming a rule but giving no reason. |

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
fast = "warn"
meant = "warn"

[thresholds]
function_lines = 40
nesting_depth = 3
params = 4
complexity = 10

[levels]
paranoid = ["src/auth/", "**/payments/*.ts"]
pragmatic = ["scripts/"]

[exclude]
paths = ["vendor/", "generated/"]
rules = ["scripts/legacy-parser.js:obvious/complexity"]

[limits]
max_file_bytes = 1048576

[baseline]
file = ".procoder-baseline.json"
```

`[rungs]` sets each rung's severity — `error` findings are must-fix, `warn`
findings advisory — and the active level modulates it. The keys are the six
rung names verbatim, `true` included: it is a bare key, not a boolean.
`[rungs] fast` and `[rungs] meant` parse and are read, and are inert in
practice: no deterministic check produces a finding at rung 5 or 6, so
promoting either changes nothing until one exists. A line
the parser cannot recognize is warned about on stderr and skipped, never
silently dropped, and so is a value it cannot read exactly. A key written
**twice** has *both* values dropped and the key left unset, with a warning
naming the file and line — a duplicate says two things and TOML permits
neither, so keeping either one would be a guess. A repeated `[table]` header
and a dotted key colliding with a table are refused the same way.

`[levels]` pins a level to the paths that earn it, so the gate follows the blast
radius rather than the session: auth, payments and crypto answer to `paranoid`
whoever is typing, and a `scripts/` directory is worth `pragmatic` even in a
strict session. Patterns are the `[exclude] paths` shapes, and a path covered by
two pins resolves to the stricter of them. A pin never restarts a session the
user turned off, and `off` is refused as a pin name — silencing a path is what
`[exclude] paths` is for, and that one reports the skip.

`[exclude] paths` reports what it costs: one stderr line per pattern per run
with the count it held back, and the pattern named — and if you name an
excluded file on the command line, that file by name. `procoder verify` also
reports a configured path exclusion whose path no longer exists, and fails on
it under `--unused-exclusions`.

`[limits] max_file_bytes` is the largest file the engine will open — anything
above it is skipped and said so on stderr, never counted clean. 1 MB is a
measured ceiling, not a preference: it is where the slowest of the six language
packs still finishes inside the hook's 2 s budget on a host three times slower
than the one that measured it. The key clamps **downward only**. A smaller value is honoured; a larger
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
the hooks: `procoder_doctrine` (the rungs at a given level, or the shorter
digest with `digest: true`), `procoder_check` (run the engine against a file),
`procoder_review` (check everything changed since a git ref, plus anything
uncommitted), and `procoder_baseline` (read the ratchet baseline). Point an
`mcpServers` config at it:

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

The server answers both eras of the protocol. MCP revision 2026-07-28 dropped
the handshake and made `server/discover` mandatory, with each request declaring
its protocol version in `_meta`; a version the server does not support is
refused with JSON-RPC error `-32022` carrying the list it does support, rather
than answered in a dialect the client cannot read. The older `initialize`
handshake still works, negotiating the client's version when it is one of
2025-11-25, 2025-06-18, 2025-03-26 or 2024-11-05.

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
