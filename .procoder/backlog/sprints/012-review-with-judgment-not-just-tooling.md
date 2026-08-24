# review with judgment, not just tooling

Status: active
Created: 2026-08-24

## Goal

Procoder gains an opinion. Every check it runs today is mechanical —
formatting, secrets, linters, hygiene — and everything requiring judgment
is left to whoever happens to be looking. By the end of this sprint
`procoder review` exists and applies five distinct stances to a change:
adversarial, edge-case, verification-gap, structure, prose. A repository
that disagrees with a lens replaces it; one whose replacement cannot be
read is told so rather than quietly getting procoder's version back.

The binary still judges nothing. It prints the lens and the scope, the
agent judges, and the findings come back through the same pipeline every
other domain uses — the contract `procoder format` has always held.

The trademark audit lands here too, ahead of the track it guards. It is
cheapest to add while there is no BMad-adjacent code to violate it, and
the boundary it holds erodes by accident rather than by decision: someone
names a new command the clearest available thing, and the clearest
available thing is the trademark.

The analysis phase and the whole BMad seam are deliberately not in this
sprint. They are separable, and shipping the review command alone is
worth a release on its own.

## Stories

<!-- Pulled with `procoder sprint pull <story-id>`. -->

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
