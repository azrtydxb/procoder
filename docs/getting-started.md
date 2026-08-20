# Getting started

Ten minutes from install to a gated, indexed, principled repository.

## 1. Install

**Claude Code** (the reference host):

```
/plugin marketplace add azrtydxb/procoder
/plugin install procoder
```

Restart or `/reload-plugins`, and the hooks are live: every file write is
checked in-turn, and every session starts with the engineering
principles.

**Any other agent**: put the binary for your platform on PATH (it ships
in `dist/` of the repo), then follow [Every agent](portability.md) for
your host's one-file setup — most hosts need nothing beyond `AGENTS.md`.

## 2. Let it install its tools

```
/procoder:init
```

`procoder doctor` surveys what THIS repository needs — formatters,
linters, scanners, index builders, by the files you actually have — and
`init` prints one install command per gap for your machine's package
managers. The agent runs them in the open; doctor confirms every gap
closed. A tool that is missing is never silently skipped: files it
would have checked are reported as **unchecked, which fails the gate**.

## 3. Onboard the codebase

New repository or one Procoder has never governed? Run the sweep:

```
/procoder:audit
```

Every domain's checks over the whole tree, aggregated into a
triage-ordered scorecard: secrets first (each needs removal AND
rotation), then blocking hygiene, then judgment calls. The skill walks
the agent through fixing incrementally — one theme per commit,
gate-clean after each — and finishes by writing the repo's `.procoder/`
files so the standard holds from here on.

## 4. Build the index

```
/procoder:index build
```

Two tiers — universal-ctags (broad) and SCIP (precise, where the
language has an indexer) — giving the agent `find`, `refs`, `callers`,
`impact`, `unused`, and `entrypoints` instead of grep. The write hook
keeps it fresh.

## 5. Work

From here the loop is the senior developer's loop:

1. **Think first** — `/procoder:spec` for anything non-trivial (the
   interview refuses to end while design gaps remain), `/procoder:plan`
   to turn the spec into executable tasks. Track them with
   `/procoder:todo` for standalone work, or seed `/procoder:backlog`
   from the spec and work the stories in `/procoder:sprint` when the
   project has a shape worth planning. Each link has a controller that
   blocks until the work is real — see
   [The quality chain](quality-chain.md).
2. **Write** — every save is format-checked in-turn; the agent gets the
   fixed content, reviews it, writes it. The binary never touches your
   files.
3. **Prove it** — `/procoder:test` runs the repository's real suite,
   with NOT run reported as NOT run. Set `[test] policy = "block"` in
   `.procoder/config.toml` and a green suite becomes part of what "done"
   means.
4. **Finish** — `/procoder:check` is the same gate CI runs. Then
   `/procoder:pr` (docs-impact answered, self-review passed, template
   filled, attribution scrubbed) and `/procoder:merge` (every check
   green, every review thread answered, reflection on anything that
   escaped, then merge and cleanup). When it is time to ship,
   `/procoder:release` lists everything standing between the tree and
   the tag, then prints the tag command for you to run.

## What you get that you didn't have

- A **"done" that means verified** — evidence-gated todos, a gate that
  counts unchecked as failing, and controllers that name exactly what is
  missing instead of arguing.
- A repo that **teaches its own agent** — principles at session start,
  rules files the repo owns, and a lessons ledger that closes each
  escaped bug's whole class.
- **One standard across every agent** you or your team run — the same
  contract file serves them all, drift-guarded by the gate.

Next: [The quality chain](quality-chain.md) ·
[Every command](commands.md) · [Configuration](configuration.md)
