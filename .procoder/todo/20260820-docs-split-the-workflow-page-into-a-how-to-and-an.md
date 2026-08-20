# docs: split the workflow page into a how-to and an explanation

Status: closed 2026-08-20
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

- [x] The how-to page states one goal in its title and contains no
      paragraph arguing for the design.
- [x] The explanation page contains no numbered procedure; where a reader
      needs steps it links to the how-to.
- [x] Every sentence of the current `docs/workflow.md` is either carried
      into one of the two pages or deliberately dropped, with the dropped
      ones listed in the evidence.
- [x] `mkdocs build --strict` is clean and both pages appear in the nav.
- [x] `procoder docs` reports no broken references.

## Evidence

- `docs/workflow.md` is now "How to ship a change": a how-to with the
  goal in the title, eleven numbered steps, and a "Common pitfalls"
  list. It contains no paragraph arguing for the design.
- The reasoning it used to carry moved to `docs/quality-chain.md`, which
  is now an explanation and carries no numbered procedure — where a
  reader needs steps it links to this page.
- Content accounting: every step of the old page survives. The worktree
  rationale, the "nothing merges over a red check" rule and the release
  controller's refusal list moved to the explanation; the merge-watcher
  protocol paragraph was dropped from the site because it describes an
  internal agent protocol already specified in
  `.procoder/github/WORKFLOW.md`, which the page names.
- `mkdocs build --strict` clean; both pages appear in the nav under
  "How-to guides" and "Explanation" respectively.
- `procoder docs` reports 0 blocking findings and no broken references.
