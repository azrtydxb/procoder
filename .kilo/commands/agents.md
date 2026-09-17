---
description: "Keep the universal agent layer in sync: per-host rule files derived from AGENTS.md, with drift blocking the gate."
---

The user invoked /procoder:agents.

Pass any user-selected `--host <name>` flags or exclusive `--all` to the
command. Without flags it uses reliable plugin context; if unknown, ask the
user which host and re-run with that explicit choice. Never choose all by
default. Review and write the printed `.procoder/hosts.json` declaration along
with the named copies. Additional hosts are additive; keep existing files.

The command below is the `procoder` binary on PATH.

procoder serves every AI coding agent from one canonical `AGENTS.md`:
rule-file hosts (Cursor, Windsurf, Cline, Kilo Code, Roo Code, Kiro,
Antigravity, Qoder, Copilot editors, Codex) each get a byte-identical
copy under their own path, and the gate blocks when any copy drifts.

1. Run `procoder agents`. For each file it reports missing or
   DRIFTED, write the printed content to the printed path — the content
   is the master plus that host's frontmatter; do not edit it by hand.
2. To change the rules themselves, edit `AGENTS.md` (the master), then
   run `procoder agents` again and rewrite every copy it lists.
   Never edit a copy directly — the next check flags it as drift.
3. Finish with `procoder check` — the same drift rules ride the gate,
   so CI agrees with what you just verified.

The full host matrix (plugin manifests, hooks, install per host) is in
the docs under "Every agent".
