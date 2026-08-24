# The analysis phase, and the seam that lets BMad plan

Status: active
Created: 2026-08-24

## Goal

The rest of the epic, in one sprint: the nine stories sprint 012 left.

Two of them finish procoder's own chain at the front. `spec check` has
always judged whether a document is complete, never whether the idea in
it is good — it will pass a thoroughly filled-in specification for the
wrong feature. The analysis phase is where an idea becomes something
worth checking, and `analyze check` holds it to the same standard a spec
is held to, because a phase whose documents nobody has to fill in is a
formality that costs time and buys nothing.

The other seven build the seam. A repository sets `[planning] method =
"bmad"` and procoder stops demanding `.procoder/specs|plans|backlog`,
reading BMad's artifacts instead. The setting moves planning and nothing
else: the gate, the suite, the release controller, the debt ledger and
the rest run identically either way, and one story asserts that with
byte-identical output across the setting rather than trusting it.

What makes this tractable is that procoder's governance never needed to
know where a plan came from. It asks whether there is one, whether it is
complete, and whether a story is done — and both worlds can answer.

## Stories

<!-- Pulled with `procoder sprint pull <story-id>`. -->

## Retro

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
