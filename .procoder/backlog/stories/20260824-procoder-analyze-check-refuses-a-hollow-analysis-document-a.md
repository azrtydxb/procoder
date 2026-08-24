# `procoder analyze check` refuses a hollow analysis document — a section left as its template comment is not a filled section — and passes a filled one.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The analysis phase is only worth having if its documents are worth
reading. A brief whose sections are still template comments has recorded
nothing, and passing it would make the phase a formality that costs time
and buys nothing.

Done means `analyze check` refuses a hollow document the way `spec check`
already refuses a hollow spec — same standard, same reason.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder analyze check` refuses a hollow analysis document — a section left as its template comment is not a filled section — and passes a filled one.

## Evidence

- `go test ./internal/analysis/ -run 'TestAHollowAnalysisIsRefused|TestCheckRefusesWhileAnyDocumentIsHollow'` — every section still carrying its template comment is a gap, an absent section is a _different_ gap from an empty one, a filled document passes, and `Check` exits 1 while any document is hollow. An empty tree exits 0: the phase is available, never required. Mutation proven: dropping StripComments lets a document that is nothing but its own template pass as COMPLETE.
