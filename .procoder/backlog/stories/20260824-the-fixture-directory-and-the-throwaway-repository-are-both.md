# Teardown, verified by absence

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 018-the-whole-campaign-re-run-from-scratch-until-nothing-new-then

## Description

The campaign creates two things that should not outlive it: a fixture
directory full of deliberate secrets and vulnerable manifests, and a
public GitHub repository holding a copy of it. Leaving either behind
turns a test artifact into a liability — a public repository of planted
credentials is exactly the thing procoder exists to complain about.

Both are removed, and the removal is verified by checking they are gone
rather than by asserting that the delete command was run. This is the
same discipline as everywhere else in the epic: a command that reports
success and did nothing is the failure mode being hunted.

The build script survives, since it is what makes the fixture
reproducible, and the campaign report survives, since it is the result.

## Acceptance criteria

- [ ] The fixture directory and the throwaway repository are both gone at
      close, verified by their absence rather than by assertion.
- [ ] The fixture build script and the campaign report remain, and the
      script still rebuilds a working fixture after teardown.

## Evidence
