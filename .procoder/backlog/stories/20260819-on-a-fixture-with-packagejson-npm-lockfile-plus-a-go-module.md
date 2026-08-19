# On a fixture with package.json (npm lockfile) plus a Go module, both runners execute and report separately.

Status: done 2026-08-19
Created: 2026-08-19
Epic: test-domain
Sprint: 001-the-test-domain-procoder-test-coverage-close-wiring-0270

## Description

Multiple ecosystems run and report separately.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture with package.json (npm lockfile) plus a Go module, both runners execute and report separately.

## Evidence

- TestDualEcosystemReportsSeparately: go+npm fixture yields two pass results. Live on this repo: `procoder test` printed `ok go pass (26 package(s))` and `---- js NOT run — package.json has no test script`.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
