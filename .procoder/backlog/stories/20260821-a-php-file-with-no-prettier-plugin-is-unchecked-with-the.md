# A PHP file with no prettier plugin is UNCHECKED with the install line and fails the gate, where it previously passed as out of scope.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

The plugin is what parses PHP; its absence was modelled as a missing style config, which made it out of scope and green. Done means it is a missing tool and fails the gate.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A PHP file with no prettier plugin is UNCHECKED with the install line and fails the gate, where it previously passed as out of scope.

## Evidence

`go test ./internal/format/ -run TestWithoutThePluginPHPIsUncheckedWithTheInstallLine`: PASS. Proved by mutation: returning OutOfScope again makes a plugin-less PHP project pass `procoder check`. Run end to end — `procoder check bug.php` printed `UNCHECKED ... the prettier PHP plugin is not installed` and exited 1.
