# A promise that names a domain must say where it lives

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

The deviation that cost sprint 021 most, in mechanical form.

Its S-3 listed nine domains that must not run in a non-adopting
repository — formatting among them — and cited nothing. Nobody had to open
a file to write that line, so nobody noticed the format loop ran BEFORE the
scope decision. Honouring that one word meant moving the config load and
scope decision above the loop, restructuring `RunWith`, and repairing four
pre-existing fixtures that had silently become non-adopting. All discovered
mid-sprint.

The rule does not verify the claim — nothing here judges prose. It requires
the claim to cite something, which puts the author in the file. That is
where the discovery happens.

## Acceptance criteria

- [x] A promise naming a domain and citing nothing is refused, and one
      that cites is accepted, per `TestAPromiseNamingADomainMustCiteIt`
      and `TestACitedDomainPromiseIsAccepted`; fails if the citation test
      in `UncitedClaims` is negated.
- [x] A promise naming no domain is left alone, per
      `TestAPromiseNamingNoDomainIsLeftAlone`; fails if the empty-set skip
      is removed and the checker becomes noise.

## Evidence

`spec.UncitedClaims` in `internal/spec/truth.go`. Three mutations applied
and watched to fail: the citation test negated, the citation test made
never to clear the rule, and the empty-set skip removed.

Measured against the spec that motivated it: the S-3 bullet listing
formatting, linting and documentation is reported, by name, at line 76.
That is defect 1 of the five, and it was previously unreachable.

Dogfooded: this rule refused a promise in its own spec — S-3 named the docs
domain, the suite and the dependency scan without citing any of them — and
the fix was to cite `procoder index build`, `procoder test` and `procoder
deps`, which is exactly the behaviour the rule is for.
