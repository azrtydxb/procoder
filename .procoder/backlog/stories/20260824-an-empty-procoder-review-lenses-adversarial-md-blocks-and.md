# An empty `.procoder/review/lenses/adversarial.md` blocks and names the file rather than falling back to the shipped lens, exiting 1 — a lens that could not load is a refusal, and a review that did not happen must not exit 0.

Status: open
Created: 2026-08-24
Epic: planning-methodology
Sprint: -

## Description

Falling back to the shipped lens when an override cannot be read would
mean a repository believing it replaced a lens that is still running
procoder's version — a silent green wearing a config file.

Done means an empty or unreadable override blocks, names the file, and
exits 1. A review that did not happen must not exit 0.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [ ] An empty `.procoder/review/lenses/adversarial.md` blocks and names the file rather than falling back to the shipped lens, exiting 1 — a lens that could not load is a refusal, and a review that did not happen must not exit 0.

## Evidence

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
