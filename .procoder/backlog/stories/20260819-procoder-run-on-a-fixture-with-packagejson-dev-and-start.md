# `procoder run` on a fixture with package.json `dev` and `start` plus a Makefile `run` target prints all three candidates, each with `source:line` evidence, most-specific first.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Most specific first.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder run` on a fixture with package.json `dev` and `start` plus a Makefile `run` target prints all three candidates, each with `source:line` evidence, most-specific first.

## Evidence

- TestRankingIsMostSpecificFirst: an explicit make run beats package.json dev, which beats start, which beats serve; language-level fallbacks (cargo, go main, compose) rank last.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
