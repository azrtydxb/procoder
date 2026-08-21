# `procoder doctor` lists every new default tool with its install line, and `procoder init` plans to install them.

Status: done 2026-08-21
Created: 2026-08-21
Epic: no-silent-green
Sprint: 009-no-silent-green-every-gate-says-when-it-did-not-run

## Description

A blocking refusal with no remedy is a wall. Done means doctor names every default tool and init can install it — and that the install actually resolves afterwards.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder doctor` lists every new default tool with its install line, and `procoder init` plans to install them.

## Evidence

Run end to end: `procoder doctor` in a TypeScript project lists `typescript-eslint ... present` via the Resolved hook (a library has no binary to find on PATH), and in a bare PHP project prints `GAP phpstan ... missing` with `procoder init` planning `composer global require phpstan/phpstan`. Two remedies that did not work were found in review and fixed: brew's llvm keg leaves clang-tidy off PATH, and the PHP plugin was installed globally while resolution walks the project — both would have installed successfully and left the file still blocked.
