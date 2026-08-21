# Checks that run themselves: the gate for the change, CI for the tree

Status: closed 2026-08-21
Created: 2026-08-21

## Goal

A person who does not know the command list still gets the checks. Today
nine of them run only when typed — the suite, dependency vulnerabilities,
complexity, debt rot, dependency freshness, environment drift, per-host
rule drift, and the spec and plan controllers — and two more run only in
CI, so a commit can carry a SAST finding whose author learns about it
after pushing.

The sprint moves the checks that are meaningful about a CHANGE into the
commit gate, and the ones that are properties of the REPOSITORY into CI.
Neither tier gets a time budget: a check runs until it answers, because
a verdict that depends on how fast the machine is, is not a verdict about
the code. Where a check is slow, it is given less to do rather than cut
off partway.

Two of these are not additions but corrections. The documentation already
says the gate blocks on agents drift, and it does not. `[test] policy =
block` reads like it governs commits, and it governs closing a story.

Done means a green gate covers what a reader would assume it covers, and
`procoder status` names anything it did not.

## Stories

<!-- pulled below -->

## Retro

**What slowed us down.** Three of thirteen stories had no test and two
had no feature, and none of that was found by the gate, by CI, or by
review. It was found by writing the close-out evidence — drafting a line
that named a test, then checking whether the test existed. It did not.
Nor did the feature: `procoder debt` returned 0 whatever it found, so the
CI step running it could only ever pass, and there was no debt leg at the
gate at all. The sprint about checks that run themselves nearly shipped
a check that ran and reported nothing.

The three missing tests were worse in kind. All three were the
assertions behind the rule the sprint turned on — the heavy checks get no
budget. The rule was stated, argued for, and built to; it was never
asserted. A rule with no test is a rule until somebody is in a hurry.

**What we change.** Evidence is written before the story is called done,
not after, and every test it names is run at the moment it is named. A
close-out is not paperwork over finished work — this sprint it was the
only check that caught anything, and it caught five things.

**Worth keeping.** Asserting the match before editing. Every mutation and
every scripted edit went through a guard that fails when the text it
expects is not there. It fired twice: once on a doc edit that silently
had not applied, and once on a mutation that would have printed `ok` from
a run where nothing was mutated. Both would have been reported as proof.

## Result

committed: 13
done: 13 (20260821-a-check-that-is-slow-still-completes-and-still-reports-its, 20260821-a-commit-adding-a-function-past-the-complexity-limit-is, 20260821-a-commit-touching-a-file-with-a-sast-finding-is-blocked-by, 20260821-a-debt-marker-with-no-revisit-condition-in-a-file-the, 20260821-a-debt-marker-with-no-revisit-condition-is-reported-at-the, 20260821-a-dependency-manifest-carrying-a-known-vulnerability-blocks, 20260821-a-failing-test-suite-blocks-the-commit-where-the-repository, 20260821-a-repository-with-no-test-setup-no-manifests-and-no-rule, 20260821-ci-runs-maintain-debt-and-deps-over-the-whole-tree-and, 20260821-procoder-status-names-every-check-the-gate-deferred-and, 20260821-rule-files-that-have-drifted-from-agents-md-block-the-gate, 20260821-sast-at-the-gate-is-given-the-changed-files-not-the-whole, 20260821-the-same-commit-produces-the-same-findings-on-a-fast-and-a)
carried: 0
