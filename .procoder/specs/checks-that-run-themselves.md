# checks-that-run-themselves

Status: COMPLETE

## Problem

Procoder has thirty-odd commands and runs a dozen of them for you. The
rest wait to be asked, and a person who does not know they exist never
asks. An audit of the hooks, the commit gate and CI found three tiers.

Automatic: formatting, lint, secrets, workflow lint, ci and infra
hygiene, the offline documentation checks, the lessons reminder, the
index refresh, pending questions, the principles at session start, and
the configuration's own findings.

CI only: `security --deep` and `docs --external`. A commit can carry a
SAST finding and a broken external link, and the author learns after
pushing.

Asked-for only, in no hook and no CI job: the test suite, the dependency
vulnerability scan, complexity limits, the debt ledger, dependency
freshness, benchmarks, environment drift, per-host rule-file drift, and
the spec and plan controllers.

Two of those are worse than merely absent. `docs/commands.md` says the
gate blocks on `agents` drift; it does not, and the only match for
`agents` anywhere near the gate is prose inside a template — so other
hosts can read rule files that have drifted from AGENTS.md while the
documentation promises otherwise. And `[test] policy = "block"` reads
like it governs commits; it governs closing a todo or a story
(cmd/procoder/main.go). A commit never runs the suite, in the gate or in
CI.

## Users

- **Someone who does not know the command list** should still get the
  checks. A quality tool that waits to be asked protects the people who
  already knew.
- **Someone committing** wants findings while the change is in hand,
  rather than after a push.
- **Someone on a large repository** needs the gate to stay usable: a
  check that takes minutes must not hold a commit hostage.

## In scope

- The asked-for checks run at the commit gate: SAST, dependency
  vulnerabilities, complexity, debt rot, environment drift, agents drift,
  and the test suite.
- Each check carries a time budget, configurable under `[timeouts]`.
- Checks are scoped to the change where the tool allows it, and say so
  where it does not.
- `procoder status` names any check the gate deferred, so nobody reads a
  green gate as coverage it did not have.

## Out of scope

- `bench`, which needs a warm machine and a baseline to compare against;
  a benchmark at commit time measures the laptop, not the change.
- `audit`, which answers about the whole tree by design.
- `doctor`, `init`, `release`, `self-upgrade`: commands a person runs
  deliberately.
- Changing what any check reports once it runs.
- Removing any command. Everything stays runnable by hand.

## Constraints

- **No silent green.** A check the gate skipped must not look like one
  that passed.
- **No wall without a door.** A budget exceeded is a refusal, so it must
  name the setting that raises it.
- The gate must stay usable on a large repository: whole-repo scans at
  every commit are not acceptable where the tool can take a file list.
- No new dependency, and no check that cannot be turned into a finding.

## Interfaces

- `.procoder/config.toml` gains `[timeouts]` with one key per check.
- No new command. `procoder check`, the hooks and CI keep their shapes.
- `procoder status` gains a line naming deferred checks when there are
  any.

## Data

- No new state. Budgets are configuration; findings are findings.

## Edge cases

- A check whose tool is absent: the existing NOT-checked path, which
  already names the install command.
- A check that exceeds its budget: NOT checked, blocking, naming the
  `[timeouts]` key that raises it.
- A repository with no test setup: the test leg reports what it always
  reported — no recognized setup — rather than failing the commit.
- A commit touching no code, only prose: the code-scoped checks report
  nothing rather than scanning the tree.
- A tool that cannot take a file list — semgrep can, `maintain` cannot —
  runs whole-repo and says so, so its cost is legible.

## Failure modes

- Adding minutes to every commit would get the gate turned off, which
  costs more than the checks are worth. Every check is budgeted and
  change-scoped where possible.
- A budget quietly skipping a check would recreate the exact silence
  this work exists to remove.
- Wiring the test suite into the gate without a budget would make a slow
  suite indistinguishable from a hung commit.

## Acceptance criteria

- [ ] A commit touching a file with a SAST finding is blocked by the gate
      at the configured severity, where previously only CI saw it.
- [ ] SAST at the gate is given the changed files, not the whole tree,
      asserted by a fixture where an untouched file carries a finding and
      the commit is not blocked by it.
- [ ] A dependency manifest carrying a known vulnerability blocks the
      commit that changes it.
- [ ] A commit adding a function past the complexity limit is blocked.
- [ ] A debt marker with no revisit condition is reported at the gate.
- [ ] Rule files that have drifted from AGENTS.md block the gate, making
      the sentence already in docs/commands.md true.
- [ ] A failing test suite blocks the commit where the repository set
      the test policy to block, and reports without blocking otherwise.
- [ ] A check that exceeds its budget reports NOT checked, blocks, and
      names the `[timeouts]` key that raises it.
- [ ] Raising a budget in `[timeouts]` lets a slow check finish, asserted
      against a deliberately slow stub.
- [ ] `procoder status` names every check the gate deferred, and says
      nothing when none were.
- [ ] A repository with no test setup, no manifests and no rule files
      commits without any new blocking finding.

## Open questions

<!-- none — decisions below -->

## Decisions

- **D-1: the heavy checks run at the commit gate, budgeted.** Findings
  are cheapest to act on while the change is in hand. The objection to
  putting them there is time, and time is answerable with a budget; the
  objection to leaving them out is that most people never run them at
  all, which is not answerable.
- **D-2: a budget exceeded is NOT checked, and blocks.** It is the same
  rule as a missing tool, for the same reason: a check that did not
  happen must not read as one that passed. It is not a wall, because the
  refusal names the `[timeouts]` key that raises the budget — a door the
  reader can open in one edit.
- **D-3: the gate blocks on agents drift, because the documentation
  already says it does.** Of the two halves, the documented behaviour is
  the right one: rule files that have drifted mean other hosts read stale
  rules while procoder reports clean. The code is what changes.
- **D-4: change-scoped where the tool allows, whole-repo where it does
  not, and legible either way.** semgrep takes a file list; `maintain`
  and `debt` answer about a tree. A check that must scan everything says
  so in its finding, so its cost is visible rather than mysterious.
- **D-5: `bench` stays out.** A benchmark at commit time measures the
  machine and the moment, not the change, and a flaky blocking check
  teaches people to bypass the gate.
