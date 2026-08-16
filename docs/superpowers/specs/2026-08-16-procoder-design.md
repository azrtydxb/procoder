# procoder — Design Spec

**Date:** 2026-08-16
**Status:** Approved for planning
**Owner:** Pascal Watteel

## 1. Purpose

procoder is a cross-platform AI-coding-agent plugin (Claude Code first, ponytail-style
multi-platform packaging) that governs **whether code is allowed to ship**. It enforces
security at trust boundaries, correctness, readability, and the removal of stale code —
continuously, in every response, not as an after-the-fact review.

It is the counterpart to [ponytail](https://github.com/dietrichgebert/ponytail): ponytail
decides *what to write* (the smallest thing that works); procoder decides *whether that
thing may ship*.

### Non-goals

- Quality scores, letter grades, or per-file badges — gameable, and nobody acts on a B+.
- Auto-fix-everything mode — rung 3 and rung 4 fixes require judgment.
- Replacing linters, formatters, or scanners. procoder orchestrates the project's own
  tooling and only falls back to built-in patterns when none is configured.
- Architectural review or design critique. procoder governs code as written.

## 2. Doctrine

### 2.1 Persona

> You are a senior developer who inherits other people's code and gets paged for their
> CVEs. You have never once been thanked for a clever line. Code is **read** far more than
> written, **attacked** more than tested, and **inherited** long after you leave.

### 2.2 The ladder

Ponytail's ladder is *stop at the first rung that holds* — a search. procoder's ladder is
**every rung must hold before it ships** — a gate. Checked in this order because the cost
of getting it wrong descends.

| # | Rung | Question | Negotiable |
|---|------|----------|------------|
| 1 | **SAFE** | Does untrusted data reach a sink unvalidated? | No |
| 2 | **TRUE** | Are errors handled and edges covered, with one runnable check left behind? | No |
| 3 | **OBVIOUS** | Would the next reader get it in one pass? | Judgment |
| 4 | **ALONE** | Did you leave a twin behind? | Judgment |

Rung 4 is the rung nobody ships, and the reason procoder exists: **a change isn't done
until the thing it replaced is gone.**

### 2.3 Rung 1 — SAFE

**Trust boundaries.** Validate at the boundary, not downstream. Every entry point
(HTTP handler, queue consumer, CLI arg, file read, env var, IPC, deserialization) is a
boundary. Validation is schema-based where the ecosystem offers it, allowlist not
denylist, and it rejects rather than coerces.

**Injection sinks.** Parameterized queries only — never string-built SQL, shell, LDAP, or
template. Output escaped for its destination context (HTML, attribute, URL, JS, shell).
No `eval`, no dynamic `require`/`import` of user-controlled paths, no unsafe
deserialization (`pickle`, Java native, YAML unsafe-load) of untrusted bytes.

**Authorization.** Enforced server-side, per-request, on the object being acted upon —
never inferred from a client-supplied role, hidden UI, or the fact that the caller reached
the endpoint. Every handler answers: who may call this, and for which resource?

**Secrets.** Never in source, never in a committed config, never in a default value, never
in a test fixture that is a real credential. Read from environment or a secret manager;
fail loudly at startup when absent rather than falling back to a baked-in default.

**Secrets and PII in logs and errors.** The most common real-world leak.

- No tokens, keys, passwords, cookies, or authorization headers in log output.
- No full request/response bodies logged at info level.
- No stack traces, SQL, or internal paths returned to a client — log the detail server-side
  with a correlation id; return the id.
- PII (email, phone, address, government id, precise location) is redacted or hashed in
  logs, analytics, and third-party telemetry.

**Dependency hygiene — a new dependency is a new trust boundary.**

- Adding one requires justification; ponytail's ladder already resists it, procoder adds
  the security argument.
- Lockfile committed. No floating/unpinned versions on a production path.
- Known-vulnerable and abandoned (>2y unmaintained) packages are flagged, using the
  project's own tooling (`npm audit`, `pip-audit`, `govulncheck`, `cargo audit`,
  `dotnet list package --vulnerable`, OWASP dependency-check).
- Install scripts, typosquat-shaped names, and packages with a single recent maintainer
  change are called out on addition.

**Crypto and transport.** Use the platform's vetted primitives; never hand-roll. No MD5/SHA1
for security purposes, no ECB, no static IVs, no `Math.random()` for tokens. TLS
verification is never disabled. Password storage uses a memory-hard KDF (argon2/bcrypt/scrypt).

### 2.4 Rung 2 — TRUE

**Errors.** Handled where the failure can lose data or corrupt state. No swallowed
exceptions, no empty `catch`, no ignored error return, no bare `except:`. Errors carry
enough context to diagnose without a reproduction. Resources are released on every path
(context managers, `defer`, `using`, `try/finally`).

**Edges.** Empty, null/absent, zero, negative, overflow, unicode, timezone/DST,
concurrent access, partial failure, retry/idempotency. Money is never a float.

**Concurrency and resources.** No unbounded queues, unclosed handles, or leaked
connections. Shared mutable state is guarded or eliminated. Cancellation/timeouts exist on
every outbound call.

**Tests — quality, not count.**

- Non-trivial logic leaves behind ONE runnable check, minimum (procoder inherits ponytail's
  rule here rather than demanding suites).
- A test must fail if the code under test is deleted or inverted. A test that passes
  against a stub is not a test.
- Assert on observable behavior, not on internal call order or private state.
- Failure paths are tested, not just the happy path.
- No `sleep()` for synchronization; no dependence on wall-clock time, network, or test
  execution order.
- Coverage percentage is never a target. "95% coverage, zero confidence" is a rung-2 failure.

### 2.5 Rung 3 — OBVIOUS

**Shape** (hook-measurable):

| Rule | Threshold |
|---|---|
| One function, one job | ~40 lines / one screen |
| Nesting depth | ≤ 3 |
| Parameter count | ≤ 4, else an options object/struct |
| Cyclomatic complexity | ≤ 10 per function |
| File cohesion | one exported concept; split when describing it needs "and" |
| Nested ternaries | never |

Thresholds are defaults. When the project configures its own
(`eslint`/`ruff`/`golangci-lint`/`clippy`), procoder reads *those* instead of imposing its own.

**Naming.**

- Names say **what**, never **how** or the type: `activeUsers`, not `userArrayFiltered`.
- No unexplained abbreviations; no single letters outside a 3-line loop or math with a
  stated convention.
- Booleans read as assertions: `isExpired`, `hasAccess` — never `flag`, `status`, `check`.
- Symmetry: opposite operations mirror (`open`/`close`, `encode`/`decode`; not
  `open`/`teardown`).
- One concept, one name, repo-wide. `user`/`account`/`member` for the same entity is a
  violation.

**Flow.**

- Guard clauses first; happy path last and un-indented.
- No surprise: a `get*` function does not write. No hidden mutation of arguments, no side
  effects behind a pure-sounding name.
- A named intermediate beats a clever one-liner: `const isEligible = …; if (isEligible)`.
- Magic values get a named constant at first repeat — or immediately when the number means
  something (`86_400`, `403`).

**Comments and docs.**

- Comment the **why**: the constraint, the surprise, the link to the ticket/RFC. Never
  restate the code.
- Every exported/public symbol gets a one-line signature doc: purpose, plus anything a
  caller cannot infer from the type (units, ownership, thread-safety, whether it throws).
- A comment that disagrees with the code is a bug — fix or delete it, never leave it.
- Stale docs are rot: rung 4 applies to READMEs and doc comments exactly as to code.

**Consistency.**

- Formatting is never argued — run the project's formatter (`prettier`, `black`, `gofmt`,
  `rustfmt`, `dotnet format`). If none is configured, propose one rather than hand-styling.
- Match the surrounding file's idiom over personal preference. A consistent codebase in a
  style you dislike is more maintainable than a mixed one you like.

### 2.6 Rung 4 — ALONE

A change isn't done until the thing it replaced is gone.

- No dead exports, unreachable branches, or unused parameters left behind.
- No commented-out code. Version control remembers; the file should not.
- No `v2` living beside `v1`, no `*_old`, `*_new`, `*_final` siblings.
- No feature flag whose branch has been settled for a release.
- No deprecation without a **removal trigger**. Migrations that must temporarily keep both
  paths carry `// procoder: remove after <condition>` — a date, a version, or a measurable
  condition. A deprecation marker without one is itself a violation.
- Stale documentation, dead config keys, unused dependencies, and orphaned test fixtures
  are all rot.
- `strict` applies rung 4 to the diff. `paranoid` applies it to every file touched.

### 2.7 Interop with ponytail

An explicit section in `SKILL.md`, because both modes may be active simultaneously.

- Ponytail chooses **what to write**. procoder decides **whether it may ship**.
- Tie-breakers: validation at trust boundaries, error handling that prevents data loss, and
  a *why*-comment on non-obvious logic are **not** "complexity smuggled back in as prose" —
  they are rungs 1–3 and they win.
- Conversely, procoder never demands an abstraction, an interface, or a doc page ponytail
  would refuse. **Documentation ≠ volume**: one line of *why* beats a paragraph of *what*.
- procoder deletes stale docs as eagerly as stale code — rung 4 and ponytail's deletion bias
  point the same direction.

### 2.8 Intensity levels

`/procoder pragmatic|strict|paranoid` — persisted, same mechanism as ponytail's level file.

| Level | Behavior |
|---|---|
| **pragmatic** | Rungs 1–2 enforced. Rungs 3–4 flagged in one line, non-blocking. |
| **strict** (default) | All four rungs enforced on code touched in this session. |
| **paranoid** | strict, plus a threat-model note on every new trust boundary, and rung 4 extended to the whole file touched rather than the diff. |

## 3. Architecture

### 3.1 Repo layout

```
procoder/
├── .claude-plugin/plugin.json      # Claude Code manifest
├── skills/
│   ├── procoder/SKILL.md           # THE doctrine — canonical source of truth
│   ├── procoder-review/SKILL.md    # diff review, 4 rungs
│   ├── procoder-audit/SKILL.md     # whole-repo audit, ranked
│   ├── procoder-rot/SKILL.md       # rung-4 specialist
│   ├── procoder-threat/SKILL.md    # rung-1 specialist
│   ├── procoder-deps/SKILL.md      # dependency hygiene
│   ├── procoder-debt/SKILL.md      # violation ledger
│   ├── procoder-gain/SKILL.md      # measured outcomes
│   ├── procoder-guard/SKILL.md     # emit pre-commit + CI
│   └── procoder-help/SKILL.md
├── commands/*.toml
├── hooks/
│   ├── procoder-activate.js        # SessionStart: inject doctrine + level
│   ├── procoder-check.js           # PostToolUse(Write|Edit)
│   ├── procoder-mode-tracker.js
│   ├── procoder-statusline.{sh,ps1}
│   └── checks/
│       ├── resolve.js              # detect project tooling, else fallback
│       ├── baseline.js             # ratchet read/write
│       ├── config.js               # .procoder.toml loader
│       ├── universal.js
│       └── lang/{ts,py,go,rust,jvm,dotnet}.js
├── scripts/sync-rules.js           # SKILL.md → every platform rule file
├── procoder-mcp/                   # MCP server (ponytail parity)
├── tests/                          # node --test + per-language fixtures
└── .cursor/ .windsurf/ .clinerules/ .kiro/ .opencode/ .agents/ AGENTS.md   # GENERATED
```

**Deviation from ponytail (deliberate):** ponytail hand-maintains ~10 copies of its
doctrine across platform directories. procoder generates all of them from
`skills/procoder/SKILL.md` via `scripts/sync-rules.js`, with a CI check that fails on
drift. Dogfooding rung 4 — no stale twins in our own repo.

### 3.2 Check hook

`PostToolUse` on `Write|Edit` → `hooks/procoder-check.js`:

1. **Scope** — only files written in this session; skip paths excluded by `.procoder.toml`.
2. **Resolve** — is `ruff`/`eslint`/`golangci-lint`/`clippy`/`dotnet format`/`gitleaks`
   configured in this project? Run it on the single written file and keep only findings
   inside the touched line range.
3. **Fallback** — no tool configured → run procoder's own pattern pack for that extension.
4. **Universal pack always runs**, regardless of tooling: secret patterns, PII/token in log
   calls, TODO/FIXME without an owner, commented-out code blocks, deprecation without a
   removal trigger. These are precisely what linters do not catch.
5. **Ratchet** — findings already present in `.procoder-baseline.json` are suppressed.
6. **Emit** — top 5 findings as `additionalContext` feedback, never a hard block. Claude
   fixes them in the same turn.

Kill switch: `PROCODER_NO_HOOK=1`. Time budget: 2s, after which the hook returns whatever
it has.

### 3.3 Ratchet / baseline

`procoder baseline` (and `/procoder-guard`) writes `.procoder-baseline.json`: a
fingerprint per existing violation (rule id + file + normalized content hash, **not** line
number, so it survives reformatting).

- Findings matching the baseline are suppressed everywhere: hook, `/procoder-review`, CI.
- New or changed code is enforced fully.
- CI fails if the baseline count **grows**; shrinking it is always allowed and is what
  `/procoder-gain` reports.

This is what makes procoder adoptable in a legacy repo instead of producing 4,000 findings
on first run and getting switched off.

### 3.4 `.procoder.toml`

```toml
level = "strict"

[exclude]
paths = ["vendor/", "migrations/", "**/*.generated.*"]

[thresholds]           # omitted keys fall back to the project's own linter, then defaults
function_lines = 40
nesting_depth = 3
params = 4
complexity = 10

[rungs]
safe = "error"
true_ = "error"   # rung 2; key avoids the TOML boolean literal
obvious = "warn"
alone = "warn"

[baseline]
file = ".procoder-baseline.json"
enforce_no_growth = true
```

## 4. Commands

| Command | Purpose |
|---|---|
| `/procoder [pragmatic\|strict\|paranoid]` | Set and persist the level |
| `/procoder-review` | Current diff against all four rungs; one line per finding |
| `/procoder-audit` | Whole repo, ranked by rung severity; writes a baseline on request |
| `/procoder-rot` | Dead exports, unreachable branches, deprecated-forever code, stale docs |
| `/procoder-threat` | Every trust boundary in the repo and what validates it |
| `/procoder-deps` | Dependency hygiene: vulnerable, abandoned, unpinned, unused |
| `/procoder-debt` | Ledger of `procoder:` markers; flags any without a removal trigger |
| `/procoder-gain` | LOC removed, boundaries hardened, baseline shrinkage |
| `/procoder-guard` | Emit a pre-commit config and CI workflow running the same checks |
| `/procoder-help` | Usage |

`/procoder-rot` and `/procoder-threat` are the differentiators — no comparable command
exists in ponytail or the mainstream review plugins.

`/procoder-guard` matters because it makes the rules survive when the agent is not in the
loop: the same check engine runs in pre-commit and CI, with the same baseline file. That is
the difference between a habit and a hope.

## 5. Output format

Findings are one line each, ranked by rung, in the shape ponytail users already read:

```
[1 SAFE]    api/users.ts:42   raw req.body.role into authz check → validate + server-side role lookup
[2 TRUE]    api/users.ts:58   error swallowed, write may be lost → propagate or log with correlation id
[3 OBVIOUS] api/users.ts:71   fn 94 lines, depth 5 → extract validate/persist/notify
[4 ALONE]   api/users.ts:6    createUserV1 still exported, no caller → delete
```

No essays. Rationale only when the finding is non-obvious, and then one clause.

## 6. Testing

- `node --test` over `hooks/` with per-language fixture files (`tests/fixtures/<lang>/`),
  each fixture pairing a violating and a clean variant.
- Golden tests for `scripts/sync-rules.js` output; CI fails on drift between `SKILL.md` and
  the generated platform rule files.
- Baseline round-trip test: violations recorded, then suppressed, then a reformat of the
  file must not resurrect them.
- Resolver test: with and without each supported linter present on PATH.

## 7. Delivery order

1. `skills/procoder/SKILL.md` (doctrine) + `procoder-activate.js` + level persistence.
2. `scripts/sync-rules.js` + generated platform rule files + drift CI.
3. Check engine: `config.js`, `resolve.js`, `universal.js`, ratchet `baseline.js`.
4. Language packs, in order: ts/js, python, go, rust, jvm, dotnet.
5. `procoder-check.js` PostToolUse wiring + statusline + mode tracker.
6. Commands: `review`, `audit`, `rot`, `threat` — then `deps`, `debt`, `gain`, `guard`, `help`.
7. `procoder-mcp/` server (ponytail parity).
8. README, examples, install docs.

## 8. Open risks

- **False positives on rung 3/4** erode trust fastest. Mitigation: rungs 3 and 4 default to
  `warn`, the hook caps at 5 findings, and the ratchet suppresses pre-existing noise.
- **Hook latency** on large files. Mitigation: single-file scope, touched-range filtering,
  2s budget.
- **Doctrine length** — a long `SKILL.md` gets skimmed by the model. Mitigation: the ladder
  table is the spine and stays at the top; the rung detail sits below it as reference,
  ordered so the first screen carries the enforceable summary.
