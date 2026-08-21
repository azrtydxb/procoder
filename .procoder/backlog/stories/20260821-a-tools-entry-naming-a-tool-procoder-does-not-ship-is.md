# A `[tools]` entry naming a tool Procoder does not ship is reported by name, lists what is available, and the default is used — the file is still checked.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

A mistyped tool name must tell somebody, and must not stop Procoder reading the code. Done means the refusal names what is available and the default still formats the file.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A `[tools]` entry naming a tool Procoder does not ship is reported by name, lists what is available, and the default is used — the file is still checked.

## Evidence

`go test ./internal/tools/ -run TestAnUnknownToolNameStillLeavesTheFileChecked`: PASS. Proved by mutation: returning nil for an unknown name makes the file "no formatter covers this file type" and the gate skips it. Run end to end — `js = "gofmt"` printed `NOT applied ... not a tool procoder ships for js — it has biome, prettier`, and the same file was still formatted by prettier.
