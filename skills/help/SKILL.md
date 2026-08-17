---
name: help
description: Reference for procoder — what it checks, its rungs, its levels, and its commands. Use when the user asks "procoder help", "what does procoder check", "procoder commands", or wants a summary of the four-rung gate.
---

# procoder:help

## Rungs

| # | Rung | Gates |
|---|------|-------|
| 1 | SAFE | Untrusted data reaching a sink unvalidated. |
| 2 | TRUE | Errors handled, edges covered, one runnable check left behind. |
| 3 | OBVIOUS | Whether the next reader gets it in one pass. |
| 4 | ALONE | Whether a stale twin was left behind. |

## Levels

- **off** — procoder does not activate.
- **pragmatic** — rungs SAFE and TRUE enforced; OBVIOUS and ALONE flagged only, non-blocking.
- **strict** (default) — all four rungs enforced on code touched this session.
- **paranoid** — strict, plus a threat-model note on every new trust boundary, and ALONE applied to whole files rather than just the diff.

`[levels]` in `.procoder.toml` pins a level to the paths that earn it
(`paranoid = ["src/auth/"]`), and that pin wins over the session level for those
files.

## Commands

| Command | Does |
|---|---|
| `/procoder:level [level]` | Set the intensity level, or show the current one if no argument is given. |
| `/procoder:help` | Show this reference. |
