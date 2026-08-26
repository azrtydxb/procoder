# Adoption is decided from the repository, never the machine

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

procoder needs to know whether a repository asked for it. Two signals,
both things a repository does deliberately: a `.procoder/` directory, or an
`AGENTS.md` that names procoder.

Never the environment. A contributor's machine is identical whether they
are in their own repository or somebody else's, so an installed binary, a
plugin, or a variable in a shell cannot be the evidence — only the
repository can.

Absence of evidence is not adoption. A repository showing no sign of having
adopted procoder is treated as not having, because the failure direction is
saying less about somebody else's code, not more.

## Acceptance criteria

- [x] A repository with `.procoder/` is adopted; one with an `AGENTS.md`
      naming procoder is adopted; one with neither is not; and one with an
      `AGENTS.md` that never mentions procoder is not.
- [x] One function answers this for every caller, so `procoder check` and
      the pre-commit hook cannot disagree.

## Evidence

`internal/gate/adoption.go` — `ScopeFor` reads the repository and nothing
else. Proved by `internal/gate/adoption_test.go`:
`TestProcoderDirectoryMeansAdopted`,
`TestAgentsFileNamingProcoderMeansAdopted`,
`TestAgentsFileAboutSomethingElseIsNotAdoption`,
`TestARepositoryWithNeitherIsUniversal`. Each mutation applied and watched
to fail.
