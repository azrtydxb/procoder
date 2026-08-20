# docs: make getting-started a tutorial that runs clean end to end

Status: open
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

- [ ] Every command in the page was run in order on a clean environment
      (a container or a fresh checkout with an empty PATH prefix) and
      produced the output the page shows.
- [ ] The page contains no alternatives, no trade-off discussion, and no
      "depending on your setup" branch; anything removed landed in a
      how-to or an explanation rather than being deleted.
- [ ] The page's opening sentence says it is a tutorial and states what
      the reader will have working at the end.
- [ ] `procoder docs` reports no broken references and
      `mkdocs build --strict` is clean.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the task open. -->
