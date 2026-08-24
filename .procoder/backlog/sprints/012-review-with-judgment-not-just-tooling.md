# Review with judgment, not just tooling

Status: closed 2026-08-24
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

**What slowed us down.** Nothing external. The cost was self-inflicted and
worth recording: the trademark audit's regex was wrong in both directions
at once — it captured "Status" from `func BmadStatus()`, missing the exact
prefix the audit exists to find, and captured "Unlinted" from
`func lintUnlinted`, a false positive on an unexported name. A greedy
`[^)]*` ate the identifier. The first mutation run passed against that
broken pattern, and passing is what surfaced it; reading the regex would
not have.

That is the third time in two sessions an audit has been written that
silently saw nothing — #153 was the same shape, and #159's first
exemption was too. The pattern is specific: source-reading audits fail
open, and a failing-open audit is indistinguishable from a passing tree
until something is deliberately broken in front of it.

**What we change.** A source-scanning audit is not finished when it
passes. It is finished when a synthetic offender of every shape it claims
to catch has been put in front of it and caught. The trademark audit has
four such probes (command arm, exported func, exported type, receiver
method) plus one proving the permissive direction — a config value and a
doctor string are left alone. That fifth probe is the one worth
generalising: an audit that only proves it catches things has not shown it
distinguishes anything.

**Worth keeping.** Checking `templates.Resolve` before writing a lens
resolver, and then not using it. The engineering ladder says look for the
helper a few files over, and it was there and nearly right — but a
template falls back to procoder's version and blocks, while a lens must
print nothing at all, because `procoder review` is not gated by the commit
gate and an agent reading output may act on it whatever the exit code
says. The ladder's rung is "does this codebase already have it", not "is
there something close enough". The right outcome was a second resolver
with a comment naming why it differs from the first.

## Result

committed: 5
done: 5 (20260824-a-repository-carrying-procoder-review-lenses-adversarial-md, 20260824-an-empty-procoder-review-lenses-adversarial-md-blocks-and, 20260824-no-procoder-owned-feature-name-contains-bmad-asserted-by-an, 20260824-procoder-review-lens-edge-case-prints-exactly-that-lens-and, 20260824-procoder-review-over-a-fixture-diff-prints-all-five-lenses)
carried: 0
