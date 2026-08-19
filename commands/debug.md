---
description: "Systematic debugging: root cause before any fix, one hypothesis at a time, and a three-strikes rule that questions the architecture instead of stacking patches."
---

The user invoked /procoder:debug with arguments:

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

The iron law: **no fixes without root-cause investigation first.** Use
this especially when a quick fix seems obvious, when you are under time
pressure, or when the previous fix didn't work — systematic is faster
than thrashing. The arguments (if any) describe the bug; start at
phase 1 with them.

Phase 1 — root cause:

1. Read the complete error: stack trace, line numbers, codes. They often
   contain the exact answer.
2. Reproduce it consistently before touching anything. Not reproducible →
   gather more data, don't guess.
3. Check what changed: `git log`/`diff` recent commits, new dependencies,
   config, environment.
4. Use the index to see the terrain: `launcher.sh index callers <fn>` and
   `refs <symbol>` for every path into the failing code — the bug's home
   may be a caller, not the crash site.
5. For multi-component failures: log what enters and exits each boundary,
   run once to find WHERE it breaks, then investigate that component.
   Trace the bad value backward to its origin, not forward from the
   symptom.

Phase 2 — pattern: find a working example of the same thing in this
codebase, read it completely (not skimmed), and list every difference —
"that can't matter" is how the difference that matters gets skipped.

Phase 3 — hypothesis: state exactly one ("I think X because Y"), test it
with the smallest possible change. Wrong → new hypothesis, never a second
change stacked on the first. Don't know → say "I don't understand X" and
investigate that; never say "this might work".

Phase 4 — fix: write the failing test FIRST (it pins the bug and proves
the fix), then the single fix — no while-I'm-here refactoring. Verify:
test passes, suite passes, gate clean.

The three-strikes rule: three failed fixes for the same bug is not bad
luck, it is evidence the architecture is wrong — each fix revealing new
coupling, or spawning new symptoms. STOP. Say so to the user, name the
pattern that keeps failing, and discuss whether to refactor it before any
fix number four.

Red flags that mean you are thrashing (return to phase 1): "quick fix for
now, investigate later" · "just try changing X" · several changes then
run the tests · "it's probably X" without evidence · fixes proposed
before any investigation. And read the user's signals: "stop guessing" or
"is that not happening?" means your evidence is missing, not their
patience.
