# docs: split the quality chain page into an explanation and how-to guides

Status: closed 2026-08-20
Created: 2026-08-20

## Description

`docs/quality-chain.md` teaches the spec/plan/backlog/todo chain, walks
through using it, and argues for why each controller refuses — three kinds
of document in one page, which ADR 0002 rules out. The chain is the
product's central idea, so this page matters more than most.

Done means: an explanation covering what the chain is and why each link
refuses, and how-to guides for the tasks a user actually performs, each
with its goal in the title.

## Acceptance criteria

- [x] The explanation page describes the chain and the reasoning, with no
      numbered procedure.
- [x] Each how-to names one goal in its title and assumes the reader knows
      what a spec is.
- [x] The worked examples keep their verbatim command output — captured
      from real runs, never fabricated, per the repo's docs rules.
- [x] `mkdocs build --strict` is clean and every new page is in the nav.
- [x] `procoder docs` reports no broken references.

## Evidence

- `docs/quality-chain.md` is now an explanation end to end: why the
  verdict lives outside the agent, why thinking comes first, why the
  project layer is separate, why evidence rather than assertion, why one
  gate, why escapes must close their class, why refusal beats advice —
  and a closing section on what refusal costs when a controller is
  wrong, which the page did not admit before.
- It carries no numbered procedure. The steps live in
  `docs/workflow.md` ("How to ship a change") and
  `docs/how-to-onboard.md`, both linked from the opening.
- The verbatim `spec check` output is kept as illustration, unchanged
  from the real run it was captured from.
- `mkdocs build --strict` clean; the page sits under "Explanation" in
  the nav, and `docs/index.md` links it from a new "Understand" section.
- `procoder docs` reports no broken references. The one anchor that
  pointed into the old heading structure
  (`#lessons-escapes-close-their-class`) was repointed to
  `#why-escapes-have-to-close-their-class`.
