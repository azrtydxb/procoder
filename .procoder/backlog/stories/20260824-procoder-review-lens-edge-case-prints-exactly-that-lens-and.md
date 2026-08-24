# `procoder review --lens edge-case` prints exactly that lens and exits 0; an unrecognised name reports the name, prints no lens at all, and exits 2 — a usage error, not a finding.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 012-review-with-judgment-not-just-tooling

## Description

A full review is the default, but a reader who already knows what they
are worried about should be able to ask one question rather than five.

Done means `--lens` selects, exits 0 when it resolved, and treats an
unrecognised name as a usage error — reported by name, nothing printed,
exit 2 — rather than silently running the other four and leaving the
reader believing they got the lens they asked for.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `procoder review --lens edge-case` prints exactly that lens and exits 0; an unrecognised name reports the name, prints no lens at all, and exits 2 — a usage error, not a finding.

## Evidence

- `go test ./internal/review/ -run TestSelectNarrowsAndNamesWhatItDoesNotKnow` — `--lens edge-case` yields exactly that lens, selection keeps the caller's order rather than the shipped one, and an unknown name comes back by name. Verified end to end: `procoder review --lens edge-case` exits 0; `--lens nope` prints `no such lens: nope — procoder has adversarial, edge-case, verification-gap, structure, prose` and exits 2, a usage error per ADR 0003. Mutation proven: Select swallowing the unknown name gives a full review and exit 0.
