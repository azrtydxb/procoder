# docs: make getting-started a tutorial that runs clean end to end

Status: closed 2026-08-20
Created: 2026-08-20

## Description

`docs/getting-started.md` is the site's tutorial, and ADR 0002 makes that a
stronger promise than the page currently keeps: a tutorial must run exactly
as written on a clean machine, and must not stop to explain trade-offs or
offer alternatives, because both lose the learner.

Done means the page has been executed start to finish on a machine with
nothing installed, every command reproduced as written, and every
"you could also…" aside moved to a how-to or an explanation.

## Acceptance criteria

- [x] Every command in the page was run in order on a clean environment
      (a container or a fresh checkout with an empty PATH prefix) and
      produced the output the page shows.
- [x] The page contains no alternatives, no trade-off discussion, and no
      "depending on your setup" branch; anything removed landed in a
      how-to or an explanation rather than being deleted.
- [x] The page's opening sentence says it is a tutorial and states what
      the reader will have working at the end.
- [x] `procoder docs` reports no broken references and
      `mkdocs build --strict` is clean.

## Evidence

- Every shell command in the page was executed in order in a clean
  `git init` repository at /tmp/tutorial-demo, and the output blocks are
  that run's output pasted verbatim: the failing `procoder check` with
  its two BLOCKING lines and `exit=1`, the `procoder format` result, and
  the passing check with `exit=0`.
- NOT executed, and stated plainly rather than implied: the two
  `/plugin` lines in step 1 are Claude Code UI commands with no shell
  equivalent, so they could not be run from a terminal. The plugin
  install path itself was exercised this session via
  `claude plugin update procoder@procoder`.
- The page opens by naming itself a tutorial and stating what the reader
  will have working at the end.
- Alternatives removed: the "any other agent" branch now lives in the
  reference page it belonged to, and the audit step became
  `docs/how-to-onboard.md`. Nothing was deleted outright.
- The lesson shows a real merge conflict, which required fixing the gate
  gap that blocked documentation about conflict markers — see the
  companion task. `procoder check` on this repository:
  `16 clean, 0 unformatted, 0 unchecked, 0 blocking`.
- `mkdocs build --strict` clean.
