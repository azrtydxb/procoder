# store: the parity harness, before the twenty-package migration

Status: open
Created: 2026-08-28

## Description

Task 7a of .procoder/plans/service-state-seam.md, split out of the
original task 7 and moved AHEAD of task 6.

The plan put every guard last. That is the wrong order for this one: the
parity harness is the only thing that would catch a behaviour change in
task 6's twenty-package migration, and building it afterwards means the
riskiest task in the plan runs with no net under it. The structural
guards genuinely do come last, because they cannot pass until task 6 is
done — so task 7 is now 7a here and 7b later.

Done means a committed set of golden outputs, proven byte-identical to
the binary built before `internal/store` existed, that fails on any
future drift.

## Acceptance criteria

- [ ] `TestMigrationOutputUnchanged` compares `procoder status`, the
      SessionStart principles hook, the Stop hook's handoff note and
      `procoder config` against committed goldens. Fails if any byte moves.
- [ ] `TestCapturesAreDeterministic` runs every capture twice and fails if
      the two differ.
- [ ] `diff` reports no difference between the goldens for status, the
      principles hook and the handoff note and the output of a binary built
      at `c4bb353`, the commit before `internal/store` existed.
- [ ] Nondeterministic values are held out of the goldens by line, not by
      dropping the line: the handoff's `generated:` timestamp and the
      config report's absolute root path are replaced, and the fact that
      each line is printed is still asserted.
- [ ] `procoder check` is clean.

## Evidence

