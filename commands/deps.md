---
description: "The dependency freshness report: what is behind, by how much, per ecosystem — judgment stays yours."
---

The user invoked /procoder:deps with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

Run:

    "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" deps

Each detected ecosystem reports through its native tool (go list -u,
npm outdated, cargo-outdated and pip where available — the tools reach
the network). Read the report as judgment, not orders: majors need a
changelog read before an update, and a NOT-checked line is a gap to
mention, never silently a clean bill. Licenses report where a tool
exists (go-licenses for Go) and say so honestly where none does.
Propose updates as backlog stories or todos — small and often beats
big and rare.
