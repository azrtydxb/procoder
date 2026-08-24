# a fixture that is not this repository, and a clean pass over it

Status: active
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
