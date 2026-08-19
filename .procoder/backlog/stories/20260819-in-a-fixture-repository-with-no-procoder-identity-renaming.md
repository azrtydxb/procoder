# In a fixture repository with no procoder identity, renaming an exported symbol with no documentation change raises the obligation naming the symbol; the same change with any doc edited raises nothing.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

Universal by construction.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] In a fixture repository with no procoder identity, renaming an exported symbol with no documentation change raises the obligation naming the symbol; the same change with any doc edited raises nothing.

## Evidence

- Live in module example.com/thing: adding exported Farewell with no doc change raised 'documentation obligation: exported symbol Farewell added in thing.go'; editing README cleared it. TestPublicSurfaceChangeWithNoDocRaisesTheObligationNamingTheSymbol green.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
