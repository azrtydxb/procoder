---
description: Map every trust boundary and what validates it.
---

<!-- DO NOT EDIT. Generated from skills/procoder/SKILL.md by scripts/sync-rules.js.
     Hand edits are overwritten and fail CI. Edit the source instead. -->

Use the procoder-threat skill. Arguments (optional path scope; default the whole
repository): $ARGUMENTS

Run the deterministic engine first, then enumerate entry points and sinks by
search, trace each entry to the sinks it reaches, and deliver the boundary table
followed by findings in the skill's one-line format.
