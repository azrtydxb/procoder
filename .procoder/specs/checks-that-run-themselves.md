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
documentation promises otherwise. The test suite is a plainer gap: nothing claims otherwise —
docs/configuration.md says the policy governs the close controllers, and
names them — but a commit never runs the suite, in the gate or in CI, so
the only thing standing between a red suite and a push is remembering.

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
- Each check runs to completion. Slowness is answered by giving a check
  less to do, never by cutting it off partway.
- Checks are scoped to the change where the tool allows it, and say so
  where it does not.
- `procoder status` names any check the gate deferred, so nobody reads a
  green gate as coverage it did not have.
- CI runs the whole-tree pass: `maintain`, `debt` and `deps` over the
  whole repository, alongside the `security --deep` and `docs --external`
  it already runs. The gate answers about the change; CI answers about
  the tree.

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
- **The same commit gets the same verdict everywhere.** A check whose
  answer depends on how fast the machine is, is not a check.
- The gate must stay usable on a large repository: whole-repo scans at
  every commit are not acceptable where the tool can take a file list.
- No new dependency, and no check that cannot be turned into a finding.

## Interfaces

- No new configuration. Nothing here is a knob, because nothing here is
  a preference.
- No new command. `procoder check` and the hooks keep their shapes; the CI
  gate job gains steps for the whole-tree checks.
- `procoder status` gains a line naming deferred checks when there are
  any.

## Data

- No new state. Budgets are configuration; findings are findings.

## Edge cases

- A check whose tool is absent: the existing NOT-checked path, which
  already names the install command.
- A wedged tool that never returns: the existing hang guards stop it and
  report NOT checked, blocking. That is a broken tool, not a slow one,
  and it is rare enough to be news when it happens.
- A repository with no test setup: the test leg reports what it always
  reported — no recognized setup — rather than failing the commit.
- A commit touching no code, only prose: the code-scoped checks report
  nothing rather than scanning the tree.
- A tool that cannot take a file list — semgrep can, `maintain` cannot —
  runs whole-repo and says so, so its cost is legible.

## Failure modes

- Adding minutes to every commit would get the gate turned off, which
  costs more than the checks are worth. The answer is scope: a check that
  reads the changed files rather than the tree is fast because it is
  doing less, not because it stopped early.
- A cutoff would recreate the silence this work exists to remove, in a
  worse form: intermittent, machine-dependent, and impossible to
  reproduce from the output.
- A slow suite is indistinguishable from a hung commit unless the gate
  says what it is running. Each check names itself as it starts.

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
- [ ] A check that is slow still completes and still reports its findings,
      asserted against a deliberately slow stub — the gate waits rather
      than reporting a verdict it did not reach.
- [ ] The same commit produces the same findings on a fast and a slow
      run, asserted by running the gate twice against stubs of different
      speeds and comparing the output.
- [ ] `procoder status` names every check the gate deferred, and says
      nothing when none were.
- [ ] A repository with no test setup, no manifests and no rule files
      commits without any new blocking finding.
- [ ] CI runs `maintain`, `debt` and `deps` over the whole tree and fails
      the job on a blocking finding from any of them.
- [ ] A debt marker with no revisit condition, in a file the commit did
      not touch, is caught by CI and not by the gate — asserted so the two
      tiers cannot silently collapse into one.

## Open questions

<!-- none — decisions below -->

## Decisions

- **D-1: the heavy checks run at the commit gate, to completion.**
  Findings are cheapest to act on while the change is in hand. The
  objection to putting them there is time, and the answer to time is
  scope — give the check less to do — never a cutoff.
- **D-2: no time budgets. A check runs until it answers.** An earlier
  draft of this spec gave each check a budget and made exceeding it a
  blocking NOT-checked with a config key to raise it. That was wrong, and
  it contradicts everything the gate has been built on. A budget makes
  the verdict depend on the machine: the same commit passes on a fast
  laptop and blocks on a slow one, the same change behaves differently in
  CI than on a developer's machine, and a finding that appears
  intermittently is one people learn to re-run rather than read. The gate
  exists to answer a question about the code; an answer that varies with
  hardware is not an answer about the code.

  A budget also has the wrong failure shape. Raising it is always easier
  than fixing what is slow, so the knob would drift upward until the
  checks never completed and nobody noticed — the silence this work
  exists to remove, arriving by a different road.

  The existing hang guards stay, and they are not budgets: they are
  generous, they exist to stop a WEDGED process rather than a slow one,
  and reaching one is news rather than routine.

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
- **D-6: the gate answers about the change, CI answers about the tree.**
  Some of these questions are not about a commit at all. Dependency
  freshness, the debt ledger and complexity across a codebase are
  properties of the repository, and asking them of every commit would
  either report the same finding forever or scan the whole tree each time
  someone edits a line. They belong in CI, where the subject is the
  repository. Where a check is meaningful about a change — SAST on the
  files touched, drift in the rule files, the suite — it belongs in the
  gate as well, and the two tiers say which is which.
