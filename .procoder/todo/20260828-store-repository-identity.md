# store: repository identity ladder and normalisation

Status: open
Created: 2026-08-28

## Description

Plan task 3 of .procoder/plans/service-state-seam.md.

The only repository key procoder has is the filesystem path from
tools.RepoRoot, and a path does not mean the same thing on two machines.
One daemon serving ten checkouts needs a stable key, and #117 flags this
as the thing most likely to be discovered too late — which is why it
lands now, with only eight RepoRoot call sites to reckon with.

The ladder is fixed: [service] repo in config.toml, then the origin
remote, then the first remote in alphabetical order, then the resolved
absolute root path. origin beats an alphabetically earlier remote
deliberately — a colleague who adds a remote named "fork" must not key
the same repository differently from everybody else.

Done means IdentityFor returns both the key and the rung that produced
it, so a surprising identity is always traceable to its source.

## Acceptance criteria

- [ ] `TestRemotes` — `gitx.Remotes` maps name to URL for a repository with an origin, and returns an empty map (not an error) for a directory that is not a repository.
- [ ] `TestIdentityNormalisation` — `git@host:o/r.git`, `https://host/o/r.git`, `ssh://git@host/o/r`, `https://HOST/o/r/` and `https://host/o/r` all produce `host/o/r`.
- [ ] `TestIdentityLadder` — config beats origin; origin beats an alphabetically earlier `fork`; `fork` plus `upstream` with no origin yields fork's URL with `Rung == \"remote\"` and `Detail == \"fork\"`; no remotes yields the resolved path.
- [ ] `TestIdentityBlankConfigKeyIgnored` — a whitespace-only `[service] repo` falls through to the next rung instead of producing an empty key.
- [ ] `TestIdentityWithoutGit` — a directory with no `.git` yields the resolved absolute path and `Rung == \"path\"`.
- [ ] `TestServiceRepoKey` — `[service] repo` parses into `Config.ServiceRepo`.
- [ ] `procoder check` is clean.

## Evidence

