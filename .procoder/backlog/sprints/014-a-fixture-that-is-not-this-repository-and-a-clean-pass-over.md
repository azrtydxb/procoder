# a fixture that is not this repository, and a clean pass over it

Status: closed 2026-08-24
Created: 2026-08-24

## Goal

Stand up a repository procoder has never seen, and find out what it says
about correct code.

The fixture is built from `git init` by a script — twelve languages,
real tests, a CI workflow that predates procoder, docs and manifests —
so that a finding can be reproduced from nothing rather than from
whatever state somebody happened to be in. Then every offline command is
pointed at it, once each, and its verdict written down.

The bar for this sprint is that a healthy repository is told it is
healthy, with three verdicts and not two: a command whose tool is
missing says NOT RUN and names the tool, and is counted with neither the
passes nor the defects.

Nothing here plants a defect. That is the next sprint, and it is the one
that proves this sprint's silence meant something — which is why this
sprint's output is a table of what ran, not a claim that all is well.

## Result

committed: 2
done: 2 (20260824-a-script-builds-a-fixture-repository-from-git-init-alone, 20260824-every-offline-command-is-invoked-against-the-healthy)
carried: 0

## Retro

**A mutation that does not fail the test proves the test, not the code.**
The first regression test for the flag guard passed with the guard's call
deleted from `run()`, because it exercised `checkFlags` directly and
never went through the dispatch. The mutation caught that, and only
because it was actually run — a second mutation the same day did not
compile, which is not a passing mutation either. Both halves matter: it
must apply, and it must build.

**Twice the interesting finding was not a finding.** `procoder deps`
reported lodash 4.18.1 as available and 4.18.1 looked invented; it is
real. `procoder ci --emit` looked like a spec marked complete with
nothing built; the spec was complete, the epic is open, and all thirteen
stories are still to do. Both would have been wrong reports filed against
the project's own record. Checking cost a minute each.

**The campaign is not exempt from the rule it is hunting.** The
brew-formula check read `tools.go` by a relative path after the script
had already `cd`'d into the fixture — grep found nothing, the loop ran
zero times, and it printed that every formula was valid. Its classifier
had done the same thing earlier by matching the word "missing" and
reading a finding about an absent PR template as a check that never ran.
An empty result is not a clean result, in the campaign's own code too.

**A fix against one repository gets checked against another.** The python
dependency gap written in the morning read `dependencies = []` as a
dependency set. Nothing in this repository has a `pyproject.toml`, so
nothing here could have found it. The fixture found it the same day.

**Anchor a patch on the function, not on the line.** `if !ok { out("no
active sprint…"); return 1 }` appears four times in `sprint.go`, and
replacing the first occurrence changed `SprintPull` — where refusing IS
correct — instead of `SprintStatus`. The pre-existing test caught it, and
the fix was to anchor on the enclosing signature.

**Installing the tools was worth more than working around them.** Five of
the six unchecked formatter rows closed, and doing it surfaced two
findings that only appear when somebody actually follows procoder's
install instructions: a formula that does not exist, and a location
procoder could not then look in.
