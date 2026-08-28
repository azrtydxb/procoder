# store: repository identity ladder and normalisation

Status: closed 2026-08-28
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

- [x] `TestRemotes` — `gitx.Remotes` maps name to URL for a repository with an origin, and returns an empty map (not an error) for a directory that is not a repository.
- [x] `TestIdentityNormalisation` — `git@host:o/r.git`, `https://host/o/r.git`, `ssh://git@host/o/r`, `https://HOST/o/r/` and `https://host/o/r` all produce `host/o/r`.
- [x] `TestIdentityLadder` — config beats origin; origin beats an alphabetically earlier `fork`; `fork` plus `upstream` with no origin yields fork's URL with `Rung == \"remote\"` and `Detail == \"fork\"`; no remotes yields the resolved path.
- [x] `TestIdentityBlankConfigKeyIgnored` — a whitespace-only `[service] repo` falls through to the next rung instead of producing an empty key.
- [x] `TestIdentityWithoutGit` — a directory with no `.git` yields the resolved absolute path and `Rung == \"path\"`.
- [x] `TestServiceRepoKey` — `[service] repo` parses into `Config.ServiceRepo`.
- [x] `procoder check` is clean.

## Evidence

- `internal/gitx/remotes.go`, `internal/store/identity.go`, and the
  `service.repo` case in `internal/config/config.go`, committed as 096fa09.
- Tests: TestRemotes and TestRemotesWithoutRepository (gitx);
  TestIdentityNormalisation, TestIdentityLadder (four subtests),
  TestIdentityBlankConfigKeyIgnored, TestIdentityWithoutGit,
  TestIdentitySource (store); TestServiceRepoKey (config).
- Mutation-checked, snapshot taken and restored around each of six:
  dropping `strings.ToLower(host)` fails TestIdentityNormalisation on
  `https://HOST/o/r/`; removing the origin branch makes the
  origin-plus-fork case return `host/me/r` instead of `host/o/r`; testing
  `cfgRepo != ""` instead of trimming yields a whitespace key with rung
  config; `filepath.Clean` in place of `filepath.EvalSymlinks` returns the
  `/var` path where the test wants `/private/var`; taking the remote name
  with `strings.Split(key, ".")[1]` loses the `up.stream` remote;
  removing the `service.repo` case leaves `ServiceRepo` empty.
- `Identity.Source()` was implemented here rather than in task 4 so the
  four rung wordings have a test of their own, TestIdentitySource, rather
  than being asserted only through the report.
- `procoder check` clean; the commit gate passed.
