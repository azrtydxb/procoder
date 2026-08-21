# `[security] sast_blocks_at = "WARNING"` makes a WARNING finding block, where the default blocks only at ERROR.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

A team with a stricter bar than the default wants the gate to enforce theirs. Done means the severity that blocks is the repository's choice, where the code had a literal.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `[security] sast_blocks_at = "WARNING"` makes a WARNING finding block, where the default blocks only at ERROR.

## Evidence

The bar was `r.Extra.Severity == "ERROR"` in internal/security/security.go and is now severityAtLeast(found, cfg.SastBlocksAt). `go test ./internal/config/ -run TestLoweringADefaultPrintsAndRaisingOneDoesNot` covers WARNING as a strengthening. An unknown severity from the tool never blocks silently — it ranks below everything and still reports.
