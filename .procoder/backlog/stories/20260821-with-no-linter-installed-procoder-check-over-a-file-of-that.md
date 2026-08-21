# With no linter installed, `procoder check` over a file of that language exits 1 and the NOT-checked line is BLOCKING, not info.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

Anyone adopting procoder needs the first green gate to mean the code was checked, not that the machine was empty. Done means a linter that could not run stops the commit, the way a missing gitleaks always has.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] With no linter installed, `procoder check` over a file of that language exits 1 and the NOT-checked line is BLOCKING, not info.

## Evidence

`go test ./internal/lint/ -run TestAMissingLinterBlocksRegardlessOfPolicy`: PASS. Proved by mutation: removing `Blocking: true` from notChecked makes it fail. Run end to end — a Go file with PATH stripped to /usr/bin:/bin printed `BLOCKING ... NOT checked — golangci-lint` and procoder exited 1, where the same command exited 0 before.
