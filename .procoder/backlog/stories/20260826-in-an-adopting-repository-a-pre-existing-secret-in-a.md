# Your own repository still hears about your own code

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

The narrowing applies to somebody else's repository, not to yours.

In an adopting repository a pre-existing secret in a changed file still
blocks. The argument for narrowing there cuts both ways — it is not this
commit's fault, and it is still a credential in your repository — and a
project that adopted procoder asked to be told. That question stays open;
this epic does not answer it by accident.

## Acceptance criteria

- [ ] In an adopting repository a pre-existing secret in a changed file
      still blocks, unchanged.

## Evidence
