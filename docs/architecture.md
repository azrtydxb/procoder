# Architecture

How Procoder is built, and the three contracts that shape every design
decision in it.

## The shape

```
┌─────────────────────────────────────────────────────────┐
│  agent (Claude Code, Cursor, Codex, … any host)         │
│    skills/commands: thin markdown callers               │
│    hooks: PostToolUse (every write), SessionStart       │
└───────────────▲───────────────────┬─────────────────────┘
                │ findings, fixed   │ invokes
                │ content, verdicts │
┌───────────────┴───────────────────▼─────────────────────┐
│  the binary (one Go executable, no runtime deps)        │
│  cmd/procoder → internal/{gate,format,security,lint,    │
│  docs,ciops,infra,maintain,codeindex,spec,plan,todo,    │
│  debt,lessons,principles,portability,host,…}            │
└───────────────┬─────────────────────────────────────────┘
                │ reads/writes only its own state
┌───────────────▼─────────────────────────────────────────┐
│  .procoder/  config.toml · PRINCIPLES.md · specs/ ·     │
│  plans/ · todo/ · github/{templates,REVIEW,LESSONS} ·   │
│  docs+security rules · index/ (derived, gitignored)     │
└─────────────────────────────────────────────────────────┘
```

The binary is the whole engine. Skills are instructions _about_ calling
it; hooks are fixed lifecycle points that call it; adapters for other
agents point at the same files. Cross-compiled per platform into
`dist/`, committed with the plugin: no npm, no network at hook time,
air-gapped installs included.

## Contract 1 — P-CONTROL: the agent stays in control

Tools compute results and hand them over; **nothing modifies code,
files, or state behind the agent's back**. The write hook does not
format your file — it hands the agent the formatted content to review
and write. `templates`, `agents`, `spec template`, `todo add` all print
content for the agent to write. The two exceptions are Procoder's own
state (`todo close` flips a Status line; the index refreshes itself),
never your code.

Why: an agent that experiences its tools as collaborators uses them; an
agent that gets silently overridden routes around its harness. And every
change stays reviewable in one place — the agent's own actions.

## Contract 2 — honesty: unchecked is never clean

A tool that is missing, times out, or returns unparseable output yields
**NOT checked** — counted by the gate as failing, never collapsed into
"no findings". This rule shows up everywhere: formatter verdicts are
clean/unformatted/unchecked (three, never two); a bare `package.json`
without a lockfile is an explicit unscannable gap; an unreadable rule
copy is UNREADABLE, not "missing"; the docs report says "offline checks
only" when it skipped the network. If a Procoder report says clean, the
check ran.

## Contract 3 — D-OVERRIDE: the repo's files win

Every domain reads its rules from `.procoder/` and the repo's version
beats the built-in default, wholesale: config policies, principles, docs
and security rules, review rubric, lessons ledger, templates. Procoder
imposes process, not opinions — a repo that wants different thresholds,
different badges, or entirely different principles writes them down and
the binary follows.

## The mirrors-and-drift pattern

Several artifacts must exist in places Procoder does not control —
GitHub reads the PR template only from `.github/`, each agent host reads
its own rule path, each host manifest carries a version. Procoder's
pattern for all of them: **one master, byte-pinned copies, drift blocks
the gate**. `mirrorSync` (PR template), `portability.Check` (ten agent
rule files + six manifest versions), `VersionSync` (README, site index,
changelog entry per release). A copy is never edited directly; the
master changes and the copies are regenerated.

## The gate is one code path

`procoder check`, `procoder git`, and CI all call the same `Collect`.
There is deliberately no way for the local gate and CI to disagree about
what the rules are — a green local gate that fails CI is defined as a
bug; when environment differences surface one (line-ending rewrites,
platform tool gaps), the fix lands in the shared path.

## The write hook, end to end

`PostToolUse` on every Write/Edit: payload on stdin → format verdict
(fixed content included if unformatted) → lint findings for that file →
markdown/doc checks if prose → secrets scan → doc-drift notes → index
refresh. Answer in the same turn, file untouched. Cost in practice:
sub-second per write.

## Testing philosophy

Every controller is pinned by refusal-path tests (the thing must say
no, and name why), every canonical list by a both-directions pin
(usage ↔ commands ↔ docs), every mirror by a drift test, and the
instruments themselves carry known-good/known-bad fixtures — a checker
that cannot catch its planted bad fixture is not trusted. Fixtures
resembling scanner targets (secrets, debt markers) are assembled at
runtime so the repository's own scans stay clean.
