# A repository lowering any default gets a line in the gate output naming the setting, its value, and the default it replaced.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

A green gate must never be able to mean "the config was loosened" without saying so — the rule the gate already lives by, applied to configuration.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A repository lowering any default gets a line in the gate output naming the setting, its value, and the default it replaced.

## Evidence

`go test ./internal/config/ -run TestLoweringADefaultPrintsAndRaisingOneDoesNot`: PASS. Run end to end — `commit_gate = "report"` produced `info relaxed: git.commit_gate = report, weaker than the default block — commits with blocking findings are no longer stopped`. It does not block: the repository chose it, and blocking would make the setting useless.
