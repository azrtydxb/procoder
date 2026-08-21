# A config.toml that cannot be parsed reports the failure, blocks, and does not silently run on defaults.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

The loader fell through its switch for any unrecognised key, so `polcy = "block"` was accepted, did nothing, and said nothing — the writer believed their policy was set.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A config.toml that cannot be parsed reports the failure, blocks, and does not silently run on defaults.

## Evidence

`go test ./internal/config/ -run TestASettingProcoderCannotApplyBlocks`: PASS — the typo and a malformed line each produce a Problem with its line number, and both block. Proved by mutation: restoring the silent fall-through accepts the typo as configured. `TestGarbageLinesAreSkippedNotGuessed` was extended to assert the report as well as the skip.
