# An empty `.procoder/review/lenses/adversarial.md` blocks and names the file rather than falling back to the shipped lens, exiting 1 — a lens that could not load is a refusal, and a review that did not happen must not exit 0.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 012-review-with-judgment-not-just-tooling

## Description

Falling back to the shipped lens when an override cannot be read would
mean a repository believing it replaced a lens that is still running
procoder's version — a silent green wearing a config file.

Done means an empty or unreadable override blocks, names the file, and
exits 1. A review that did not happen must not exit 0.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] An empty `.procoder/review/lenses/adversarial.md` blocks and names the file rather than falling back to the shipped lens, exiting 1 — a lens that could not load is a refusal, and a review that did not happen must not exit 0.

## Evidence

- `go test ./internal/review/ -run TestAnUnreadableOverrideBlocksAndDoesNotFallBack` — an empty override is exactly one blocking finding naming the file, and the lens set comes back with four entries rather than five: procoder does NOT substitute its own. Verified end to end: `procoder review` printed the block and no lens at all, exit 1. Mutation proven: returning the shipped lens alongside the finding, the way templates.Resolve does, restores the count to five and prints procoder's stance under the repository's name.
