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

<!-- What slowed us down this sprint. -->

<!-- What we change next sprint because of it. -->

<!-- One adaptation from this sprint worth keeping. -->
