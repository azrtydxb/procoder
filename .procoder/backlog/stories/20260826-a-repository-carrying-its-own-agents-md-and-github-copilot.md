# procoder does not claim a file it never wrote

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

The finding that reads as arrogance: procoder told a project that its
own `AGENTS.md` and `.github/copilot-instructions.md` — written by that
project, for that project, long before procoder arrived — had "drifted from
AGENTS.md" and demanded they be rewritten.

"Drifted" is the wrong word for a file that was never procoder's. In a
repository that has not adopted procoder, those files get no finding of any
kind: not drift, not missing, nothing.

## Acceptance criteria

- [x] A repository carrying its own `AGENTS.md` and
      `.github/copilot-instructions.md`, having not adopted procoder, is
      told nothing about either file.

## Evidence

`TestANonAdoptingRepositoryIsNeverToldItsFilesDrifted` runs the whole gate
over a fixture holding somebody else's `AGENTS.md` and
`.github/copilot-instructions.md`, and requires the word "drift" to appear
nowhere. Killed by routing the universal branch through
`gitcmd.CollectFor`.
