# release-discipline

Status: draft

## Problem

Releasing is a hand-run checklist: bump every version-bearing file,
write the changelog entry, rebuild artifacts, verify nothing disagrees.
procoder enforces discipline everywhere except its own most error-prone
ritual — three releases today were artisanal, and any repo using
procoder has the same unencoded ritual. A missed file ships a stale
version and only CI (at best) notices.

## Users

- Pascal (or any maintainer) cutting a release: one command that says
  ship or names what is missing.
- The agent, which gets a checklist it cannot skip parts of.

## In scope

- `procoder release <version>` — the pre-tag controller. It verifies,
  in one pass, ALL of:
  - every file listed under `[release] files` in config.toml
    (D-OVERRIDE, no default guessing) contains the literal version;
  - CHANGELOG.md has a `## <version>` heading;
  - the git working tree is clean (releases come from committed state);
  - the gate is clean;
  - the test suite passes when `[test] policy = "block"` is set.
    Every failure is listed (never just the first); exit 1 while any
    remain. On success it prints the tag command
    (`git tag -a v<version> -m <version>`) for the agent to run —
    P-CONTROL, the binary tags nothing.
- `procoder release` with no argument reports the newest version found
  in CHANGELOG.md and whether the checklist passes for it.
- procoder's own config.toml lists its nine version-bearing files.

## Out of scope

- Bumping files or writing the changelog (the agent edits; the binary
  judges).
- Publishing, pushing tags, GitHub Releases, artifact upload.
- Semver validation beyond a plausible `N.N.N`-shaped string.
- Release trains, branches, or backports.

## Constraints

- Pure Go stdlib; package internal/release.
- P-CONTROL: read-only verification plus a printed next step.
- Honesty: a file that cannot be read is a failure named as unreadable,
  not skipped; `[release] files` unset means the version-sync leg says
  it verified nothing (out loud) rather than passing silently.
- Reuses gate.Run and testrun.Suite in-process.

## Interfaces

- `procoder release [<version>]`, exit 0 ship / 1 blocked / 2 usage.
- config: `[release] files = ["plugin.yaml", ...]` list of repo-root
  relative paths.
- Usage text, docs.Commands, docs site, commands/release.md skill +
  OpenCode twin.

## Data

- No state written; config read from config.toml.

## Edge cases

- Version present in a listed file only as part of a longer string
  (0.27.0 inside 10.27.01) — matching is substring by design and said
  in the docs; the changelog heading check is exact.
- CHANGELOG missing entirely → that leg fails naming the file.
- Dirty tree includes untracked files → still dirty (a release must
  not depend on uncommitted anything).
- `[release] files` naming a missing file → failure line for that file.
- Version argument malformed ("v1.2" / "banana") → exit 2 with the
  expected shape.

## Failure modes

- git absent → the clean-tree leg reports NOT verified and counts as
  failing (unknown is never clean).
- Gate or suite infrastructure broken → that leg fails with the
  underlying message, never passes silently.

## Acceptance criteria

- [ ] On a fixture with two listed files, one stale, `procoder release
1.2.3` lists exactly the stale file, the missing changelog
      heading, and the dirty tree — all in one output — and exits 1.
- [ ] After fixing all three, it prints the tag command and exits 0,
      having tagged nothing.
- [ ] With `[release] files` unset the output states version-sync
      verified nothing.
- [ ] `procoder release` (no argument) reads the newest changelog
      version and reports the checklist for it.
- [ ] procoder's own config lists its version files and `procoder
release <current>` passes on a clean tree.

## Open questions

<!-- none — decisions recorded above -->
