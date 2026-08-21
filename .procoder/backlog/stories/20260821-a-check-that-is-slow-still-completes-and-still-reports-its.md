# A check that is slow still completes and still reports its findings, asserted against a deliberately slow stub — the gate waits rather than reporting a verdict it did not reach.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

A developer on a slow laptop and a developer on a fast one must get the
same answer about the same commit. A check cut off partway and reported
anyway turns the verdict into a fact about the machine, and the first
time someone's commit passes on their machine and fails on a colleague's,
they stop believing any of it.

So the heavy legs get no budget. Done means a check that takes seconds
still finishes and still reports what it found, asserted against a stub
that is deliberately slow rather than against a real tool that might be
fast on the test runner.

The ceiling that exists is a hung-process net, not a budget: when it
fires it says the check was NOT run and blocks. Waiting forever and
passing silently are both worse than saying so.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A check that is slow still completes and still reports its findings, asserted against a deliberately slow stub — the gate waits rather than reporting a verdict it did not reach.

## Evidence

- `go test ./internal/security/ -run TestASlowCheckStillCompletes` — a semgrep stub delayed two seconds still returns its finding intact, and the call waited 2.55s rather than answering early. Red against a 1s ceiling returning what it reached: "semgrep output unreadable — SAST was NOT run". (#137)
