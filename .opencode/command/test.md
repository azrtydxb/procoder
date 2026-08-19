---
description: "Run the repository's actual test suite: every ecosystem's canonical runner, reported honestly — NOT run is never green."
---

The user invoked /procoder:test with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

1. Run `procoder test` (add `--coverage` for the percentage where the
   runner measures it natively; pass paths to narrow Go packages and
   pytest targets). Every detected ecosystem runs: go test, cargo test,
   the package.json test script, pytest, gradle/maven.
2. Read the verdicts honestly: `ok` passed, `FAIL` names what broke —
   fix the code or the test, never delete the assertion to get green.
   `----` means NOT run (no runner, no test script): that is a gap, not
   a pass — say so when reporting.
3. Exit codes: 0 all green, 1 something failed, 2 nothing could run.
4. With `[test] policy = "block"` in .procoder/config.toml, todo close
   and backlog story close run this suite and refuse while it is red —
   the suite being green is part of "done", not a separate favor.
5. Re-run after every fix; report the final verdict lines to the user.
