# A criterion says what would make it fail

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

The project's own mutation discipline, applied to the criterion rather than
to the test. `procoder test` already expects a test to name the mutation
that must make it fail; a criterion was held to no such standard.

Two of sprint 021's five deviations were criteria that could not fail. One
promised that narrowing junk findings to the diff was prevented — a failure
that cannot happen, because those findings carry no line number and the
narrower keeps line-less findings by construction. The other promised that
a typo in the scope override falls back to detection, on a fixture where an
accepted typo and a correct fallthrough are indistinguishable, because
`parseScope` returns Adopted as its zero value.

Both were green whatever the code did. Stating the falsifier is what
surfaces them: you cannot say what change would break a criterion without
constructing the case that separates pass from fail, and when you cannot,
that is the answer.

## Acceptance criteria

- [x] A criterion with no falsifier is refused, in any of the accepted
      phrasings, per `TestACriterionWithNoFalsifierIsReported` and
      `TestACriterionNamingItsFalsifierIsAccepted`; fails if `falsifierRe`
      is cut to a single phrasing.
- [x] Both sprint-021 criteria that could not fail are reported by line,
      per `TestAllFiveMotivatingDefectsAreReported`; fails if
      `CriteriaWithoutFalsifiers` is made to return nil.

## Evidence

`spec.CriteriaWithoutFalsifiers` in `internal/spec/truth.go`. Several
phrasings are accepted — `fails if`, `proved by`, `breaks if` and others —
because a rule accepting one exact form is satisfied by pasting that form
rather than by thinking. Two mutations applied and watched to fail.

Measured against the spec that motivated it: 11 criteria name no falsifier,
including the junk/oversized one at line 190 and the scope-override one at
line 196 — defects 2 and 5 of the five, both previously unreachable.

A criterion that names a test is exempt: the test carries its own `proved
by:` and `procoder test` is what asks for it, so demanding the clause here
too would be the same question twice — and a rule that asks twice is one
people satisfy by pasting. That exemption is what keeps this affordable,
and `TestACriterionNamingNeitherIsStillCaught` pins that it does not
swallow the rule. Drafts only, so no existing spec is rewritten.
