---
description: "The state of play, computed fresh: branch, dirty files, the active sprint, open work, index freshness."
---

The user invoked /procoder:status.

Run:

    "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" status

Every line is a computed fact, not a guess — and a fact that could not
be read says so rather than defaulting to something comfortable. Read
it before deciding what to do next: the active sprint and its open
stories are the committed work, open todo tasks are the standalone
list, and a stale index means line numbers cannot be trusted until you
rebuild.

The same block is injected at session start, so a resumed session opens
knowing where the project stands. `.procoder/state/handoff.md` holds
the last session's version of it, plus any notes the previous session
left under its Notes heading.
