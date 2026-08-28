# store: the parity harness, before the twenty-package migration

Status: closed 2026-08-28
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

- [x] `TestMigrationOutputUnchanged` compares `procoder status`, the
      SessionStart principles hook, the Stop hook's handoff note and
      `procoder config` against committed goldens. Fails if any byte moves.
- [x] `TestCapturesAreDeterministic` runs every capture twice and fails if
      the two differ.
- [x] `diff` reports no difference between the goldens for status, the
      principles hook and the handoff note and the output of a binary built
      at `c4bb353`, the commit before `internal/store` existed.
- [x] Nondeterministic values are held out of the goldens by line, not by
      dropping the line: the handoff's `generated:` timestamp and the
      config report's absolute root path are replaced, and the fact that
      each line is printed is still asserted.
- [x] `procoder check` is clean.

## Evidence

- `internal/store/golden_test.go` and four goldens under
  `internal/store/testdata/golden/`, committed as 994ed5e.
- Parity proof, run against a worktree at `c4bb353` built to
  `pc-old`: `diff` reported no difference for `status.txt`,
  `principles-hook.txt` and `handoff.txt`. The worktree has been removed;
  the check is reproducible with `git worktree add <dir> c4bb353` and
  `go build ./cmd/procoder` inside it.
- `config.txt` is deliberately NOT a parity golden. Task 4 changed that
  output on purpose — the identity line is new — so it is captured from
  current code and guards drift from here.
- TestCapturesAreDeterministic paid for itself on first run: the handoff
  note stamps `generated:` with the wall clock, so the first goldens failed
  a second after being written. Both that line and the config report's
  absolute root path are now replaced by line rather than dropped, so the
  presence of each line is still asserted.
- The harness is `package store_test`, not `package store`: it drives
  status, principles, the stop hook and the config report, all of which
  import internal/store, so an in-package test would be an import cycle.
- KNOWN, deliberate exclusion: `procoder check`, PreToolUse and
  PostToolUse are not captured. They exec gitleaks, semgrep,
  golangci-lint or the test runner, whose presence and version vary by
  machine, so a golden of those pins one laptop rather than procoder's
  behaviour. Recorded in the spec's Out of scope, not left silent.
- `procoder check` clean; `go test ./...` green.

