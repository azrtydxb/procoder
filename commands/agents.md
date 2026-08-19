---
description: "Keep the universal agent layer in sync: per-host rule files derived from AGENTS.md, with drift blocking the gate."
---

The user invoked /procoder:agents.

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

procoder serves every AI coding agent from one canonical `AGENTS.md`:
rule-file hosts (Cursor, Windsurf, Cline, Kilo Code, Roo Code, Kiro,
Antigravity, Qoder, Copilot editors, Codex) each get a byte-identical
copy under their own path, and the gate blocks when any copy drifts.

1. Run `launcher.sh agents`. For each file it reports missing or
   DRIFTED, write the printed content to the printed path — the content
   is the master plus that host's frontmatter; do not edit it by hand.
2. To change the rules themselves, edit `AGENTS.md` (the master), then
   run `launcher.sh agents` again and rewrite every copy it lists.
   Never edit a copy directly — the next check flags it as drift.
3. Finish with `launcher.sh check` — the same drift rules ride the gate,
   so CI agrees with what you just verified.

The full host matrix (plugin manifests, hooks, install per host) is in
the docs under "Every agent".
