# Either mode can be forced

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

The detection is evidence, not a verdict nobody can argue with.

`[gate] scope` in `.procoder/config.toml` forces either mode — readable
only where `.procoder/` exists, which is itself adoption.
`PROCODER_GATE_SCOPE` does the same for a repository that cannot carry
config: a fork somebody is about to submit upstream, where adding
`.procoder/` to the tree would itself be a change they do not want to
make.

## Acceptance criteria

- [ ] `[gate] scope = "universal"` in an adopting repository reduces it to
      the universal checks.
- [ ] `PROCODER_GATE_SCOPE=adopted` in a non-adopting repository runs
      everything.

## Evidence
