# Run against `.procoder/specs/adoption-aware-gate.md` — the spec whose deviations motivated this — the check reports the criteria that name no observable. Which of the five defects are mechanically reachable is stated in the evidence rather than assumed, so a defect this cannot reach is recorded as such rather than quietly dropped.

Status: done
Created: 2026-08-26
Epic: specs-checked-for-truth
Sprint: 024-a-spec-s-claims-are-checked-by-something-other-than-whoever-wrote-them

## Description

A check that cannot find the defects it was built for has not been shown to work.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Run against `.procoder/specs/adoption-aware-gate.md` — the spec whose deviations motivated this — the check reports the criteria that name no observable. Which of the five defects are mechanically reachable is stated in the evidence rather than assumed, so a defect this cannot reach is recorded as such rather than quietly dropped.

## Evidence

`TestTheSpecThatMotivatedThisIsCaughtByIt` runs against `.procoder/specs/adoption-aware-gate.md` and requires it to report.

Measured: **9 of that spec's criteria name no observable**, including the README/documentation one and the suite one — defects 3 and 4 of the five that motivated #186.

The other three are NOT mechanically reachable, and are recorded here rather than counted as covered: a claim about WHERE in the code a decision is made (S-3 naming formatting, without anyone checking that the format loop ran before the scope decision); a criterion describing a failure that cannot happen (narrowing junk findings, which carry no line number); and a zero-value collision that made a fixture unable to distinguish an accepted typo from a correct fallthrough. Each needs reasoning about behaviour, which this deliberately does not attempt.

Citations: 0 unresolved in that spec — it cited accurately, and its problem was elsewhere.
