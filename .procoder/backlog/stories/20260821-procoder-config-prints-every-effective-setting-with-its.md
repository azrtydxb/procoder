# `procoder config` prints every effective setting with its source, and a repository with no config.toml shows every source as default.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

Configurability without visibility is worse than none: a person reading an unfamiliar repository has to be able to ask which defaults are still in force.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder config` prints every effective setting with its source, and a repository with no config.toml shows every source as default.

## Evidence

`go test ./internal/config/ -run TestARepositoryWithNoConfigIsAllDefaults`: PASS — every source reads `default` and Report exits 0. Run end to end on this repository: it printed nine settings with their file and line, marking `git.max_file_mb = 10 ← relaxed from 5`. A false relaxation was caught while building it — bench.threshold's real default is 10, not the field's zero value, and calling an explicit 10 a relaxation would have warned at every repo that set the default deliberately.
