# procoder's own .procoder/config.toml carries the block policy and the repository's suite passes under `procoder test`.

Status: done 2026-08-19
Created: 2026-08-19
Epic: test-domain
Sprint: 001-the-test-domain-procoder-test-coverage-close-wiring-0270

## Description

This repository dogfoods the policy.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] procoder's own .procoder/config.toml carries the block policy and the repository's suite passes under `procoder test`.

## Evidence

- .procoder/config.toml now carries [test] policy = "block"; `procoder test` on this repo: go pass (26 packages), js honestly NOT run.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
