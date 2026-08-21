# Checks that run themselves: the gate for the change, CI for the tree

Status: active
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

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
