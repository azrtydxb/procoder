# docs: split the quality chain page into an explanation and how-to guides

Status: open
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

- [ ] The explanation page describes the chain and the reasoning, with no
      numbered procedure.
- [ ] Each how-to names one goal in its title and assumes the reader knows
      what a spec is.
- [ ] The worked examples keep their verbatim command output — captured
      from real runs, never fabricated, per the repo's docs rules.
- [ ] `mkdocs build --strict` is clean and every new page is in the nav.
- [ ] `procoder docs` reports no broken references.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the task open. -->
