# How to install Procoder without the plugin

**A how-to guide.** Goal: run Procoder with an agent other than Claude
Code — Cursor, Windsurf, Codex, Copilot, OpenCode, Cline, or anything
that reads `AGENTS.md`.

Using Claude Code? Do the [tutorial](getting-started.md) instead. The
plugin carries the binary and wires the hooks for you, and none of this
page applies.

## 1. Put the binary on PATH

The repository ships a prebuilt binary per platform under `dist/`. No
runtime, no npm, no network at hook time.

```
git clone https://github.com/azrtydxb/procoder
export PATH="$PWD/procoder/dist/darwin-arm64:$PATH"
procoder version
```

Pick the directory that matches your machine: `darwin-arm64`,
`darwin-amd64`, `linux-amd64`, `linux-arm64`, or `windows-amd64`.

Make the `PATH` line permanent in your shell profile, or copy the binary
somewhere already on `PATH`.

## 2. Write the agent contract

In the repository you want governed:

```
procoder agents
```

This prints `AGENTS.md` — the always-on contract — plus the per-host
rule file your agent reads, and it tells you the path for each. **It
writes nothing**; you review the content and write the files.

Most hosts need only `AGENTS.md`. Cursor, Windsurf, Cline, Kilo Code,
Roo, Kiro, Antigravity, Qoder, Copilot and Codex each take one extra
file at their own path — see [Every agent](portability.md) for the full
table.

## 3. Close the tool gaps

```
procoder init
```

Prints one install command per missing formatter, linter, scanner, and
index builder that **this** repository needs. Add `--yes` to run them
and re-check that every tool answers.

A missing tool is never silently skipped: files it would have checked
are reported **unchecked, and unchecked fails the gate**.

## 4. Wire the commit gate

```
procoder hook install-git
```

Prints the pre-commit hook that runs `procoder check` before a commit
lands. Without it, nothing stops a commit — the gate is a command your
agent runs, not a thing that intercepts you.

## Common pitfalls

- **Do not** skip `procoder agents`. Without the contract file the agent
  has the binary and no instructions about when to run it, which is most
  of the value gone.
- **Do not** edit a generated per-host rule file directly. `AGENTS.md`
  is the master; regenerate the copies with `procoder agents`, and the
  gate blocks on drift between them.
- **Do not** assume the plugin and the manual install differ in
  behaviour. It is the same binary and the same rules — only the wiring
  differs.

## Next

- [Ship a change](workflow.md) — the daily sequence. Where it names a
  `/procoder:` command, run the equivalent binary subcommand instead:
  `/procoder:check` is `procoder check`.
- [Every agent](portability.md) — the per-host file table.
- [Command reference](commands.md) — every command and its flags.
