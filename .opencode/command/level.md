---
description: Set the procoder intensity level: pragmatic, strict, or paranoid.
---

<!-- DO NOT EDIT. Generated from skills/procoder/SKILL.md by scripts/sync-rules.js.
     Hand edits are overwritten and fail CI. Edit the source instead. -->

The user invoked /procoder:level with arguments: $ARGUMENTS

If the arguments name a level (pragmatic, strict, paranoid, or off), the
UserPromptSubmit hook has already persisted it — confirm the new level in one
line and state what changed:
- pragmatic: rungs SAFE and TRUE enforced; OBVIOUS and ALONE flagged only.
- strict: all four rungs enforced on code touched this session.
- paranoid: strict, plus a threat-model note on every new trust boundary, and
  ALONE applied to whole files rather than just the diff.

If no level was given, report the current level and list the options. One line
each. Do not restate the doctrine.
