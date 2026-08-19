---
description: "Update the procoder plugin from the marketplace and verify the new version end to end."
---

The user invoked /procoder:update.

1. Update the installed plugin from the marketplace:

       "$HOME/Library/Application Support/Claude/claude-code/"*/claude.app/Contents/MacOS/claude plugin update procoder@procoder

   (or plain `claude plugin update procoder@procoder` when `claude` is on
   PATH). It prints the old and new version.

2. Read the new install path from `~/.claude/plugins/installed_plugins.json`
   and verify the shipped binary answers:

       <installPath>/hooks/procoder version

3. Test the new version by DIRECT invocation from that path — `doctor`,
   `check`, and a hook probe (pipe a PostToolUse JSON payload into
   `procoder hook post-tool-use`). The running session keeps the previous
   version's hooks until the user runs /reload-plugins — say so in your
   report rather than claiming the session is updated.
4. Report: previous version, new version, and what was verified.
