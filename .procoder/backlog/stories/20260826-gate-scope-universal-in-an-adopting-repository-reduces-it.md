# Either mode can be forced

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

The detection is evidence, not a verdict nobody can argue with.

`[gate] scope` in `.procoder/config.toml` forces either mode — readable
only where `.procoder/` exists, which is itself adoption.
`PROCODER_GATE_SCOPE` does the same for a repository that cannot carry
config: a fork somebody is about to submit upstream, where adding
`.procoder/` to the tree would itself be a change they do not want to
make.

## Acceptance criteria

- [x] `[gate] scope = "universal"` in an adopting repository reduces it to
      the universal checks.
- [x] `PROCODER_GATE_SCOPE=adopted` in a non-adopting repository runs
      everything.

## Evidence

`TestForcingUniversalInAnAdoptingRepositoryReducesTheGate` — forcing it
changes the checks, not just the printed word. Plus
`TestConfigScopeOverridesEverything` and `TestEnvironmentOverridesDetection`
for the precedence, and `TestAnUnreadableOverrideFallsBackToDetection` for
a typo.

That last fixture had to be a non-adopting repository: `parseScope` returns
`Adopted` as its zero value, so in an adopting one an accepted typo and a
correct fallthrough are indistinguishable. The first version passed for the
wrong reason. #186.
