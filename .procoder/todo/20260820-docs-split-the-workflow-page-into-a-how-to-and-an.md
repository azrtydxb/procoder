# docs: split the workflow page into a how-to and an explanation

Status: open
Created: 2026-08-20

## Description

`docs/workflow.md` serves two readers at once. Part of it is a how-to — the
daily sequence a competent user follows from spec to tagged release. The
rest explains why the sequence is shaped that way and what each refusal
buys. Under the documentation system adopted in ADR 0002 that is two
pages, and a reader arriving with one need currently pays for both.

Done means: a how-to with the goal in its title and numbered steps that
assume competence, an explanation that carries no steps and links to the
how-to instead, both reachable from the nav, and no content lost in the
split.

## Acceptance criteria

- [ ] The how-to page states one goal in its title and contains no
      paragraph arguing for the design.
- [ ] The explanation page contains no numbered procedure; where a reader
      needs steps it links to the how-to.
- [ ] Every sentence of the current `docs/workflow.md` is either carried
      into one of the two pages or deliberately dropped, with the dropped
      ones listed in the evidence.
- [ ] `mkdocs build --strict` is clean and both pages appear in the nav.
- [ ] `procoder docs` reports no broken references.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the task open. -->
