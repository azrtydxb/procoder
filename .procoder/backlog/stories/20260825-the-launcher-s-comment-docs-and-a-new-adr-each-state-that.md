# The contract that changed is written down

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 020-the-binaries-leave-the-tree-and-ci-builds-them

## Description

`hooks/launcher.sh` says today: "marketplace install, no runtime, no
network." That was a real property and it is being spent.

The launcher's own comment, the documentation, and a new ADR each say what
is true afterwards: the first run fetches over the network, what happens
when it cannot, and why the trade was made. An ADR because a changed mind
supersedes rather than rewrites, and somebody will ask why procoder
reaches the network at session start.

## Acceptance criteria

- [ ] The launcher's comment, `docs/`, and a new ADR each state that the
      first run fetches over the network, and what happens when it
      cannot.
- [ ] The ADR records what was given up — offline first-run, and
      third-party verifiability — not only what was gained.

## Evidence
