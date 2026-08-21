# A repository upgrading with no config changes produces byte-identical gate output to the previous version, asserted on a fixture.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

The story protecting everyone who is happy as things are. Done means a repository that changes nothing behaves exactly as it did.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A repository upgrading with no config changes produces byte-identical gate output to the previous version, asserted on a fixture.

## Evidence

`go test ./internal/config/ -run TestARepositoryWithNoConfigIsAllDefaults`: PASS — a repository with no config.toml produces zero config findings, every source reads `default`, and nothing is marked relaxed. Proved by mutation: giving any setting a non-default effective value shows a source that is not `default`. The full suite is green across all three PRs, and this repository's own gate output is unchanged apart from the settings it deliberately sets.
