# The GitHub-dependent commands, against a throwaway public repository

Status: done 2026-08-24
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 017-procoder-against-a-real-github-repository-ci-and-a-release

## Description

Four surfaces reach the GitHub API and a local fixture cannot answer for
any of them: `ci --runs` reads workflow history, `copilot-leak` reads
review comments, `docs --external` checks published Pages, and the
release path tags, builds and publishes. Calling these covered on the
strength of a local run would be this campaign committing the exact
failure it was built to find.

So the fixture gets pushed to a throwaway public repository, its CI runs
for real, and a real version is tagged and released through the workflow.
Public rather than private because Actions minutes are free there and
this will run many times.

The repository is named as a fixture so anyone who finds it knows what
it is, and it is deleted when the epic closes — verified by its absence,
which is a separate story.

## Acceptance criteria

- [x] `ci --runs`, `copilot-leak`, `docs --external` and a tagged release
      are run against a throwaway public repository, and each either
      passes or is reported with what it did.
- [x] The fixture's CI completes at least one full run and its result is
      read back through `procoder ci --runs`, not through the web UI.
- [x] A release is cut end to end and the published artifact is verified
      by download, the way this repository's own releases are.

## Evidence

Against `azrtydxb/procoder-e2e-fixture`, public, created for this and
deleted in sprint 018.

- **`ci --runs`** read three real states back. Immediately after the push:
  "ci: in progress — started less than a minute ago, no conclusion yet".
  After the run landed: "ci: success — 1m ago". After a deliberately
  failing test was pushed: "ci: failure — less than a minute ago / failing
  job(s): test", naming the job. And after a local commit: "HEAD is not
  pushed — CI cannot have seen it", which qualifies the success verdict
  instead of letting it stand for a commit CI never saw.
- **`copilot-leak`** answers against the live API: "no findings since
  24h0m0s" on a repository with no reviewed pull requests.
- **`docs --external`** reports "GitHub Pages is not enabled for this
  repository — the docs site is not being served" rather than passing
  quietly, and blocks on a real 404 with the URL and the line: `dead
external link: * [404] <…/no-such-file-e2e-campaign.md> (at 23:3)`.
- **The release, end to end.** The controller refused twice first — no
  `## 0.2.0` changelog heading, then a dirty tree — and warned
  "version-sync verified nothing" until `[release] files` was set, rather
  than reporting a sync it had not performed. When ready it PRINTED the
  tag command and created no tag (`git tag` was empty before and after).
  Tagged, pushed, published with a binary and a checksum manifest, then
  downloaded into a fresh directory: published hash
  `a852ab949087412fba9114d6929dde28fd73ff87cddffaefa83a3b7568357f8c`
  matched the downloaded file, and the binary ran. Empty hashes refuse
  rather than compare, so a failed download cannot print a match.
- **One defect found and fixed** (`c0d7b72`): asked for 0.2.0 again, after
  it was tagged and published, the controller answered "ready — tag it:
  git tag -a v0.2.0" — a command that answers "fatal: tag 'v0.2.0'
  already exists".

**Not covered, and recorded as not covered:** the Pages health check's
positive path. Pages is not enabled on the fixture, so procoder's
"not enabled" branch is exercised and the "enabled but stale" branch is
not. Enabling Pages on a throwaway repository to test one branch was
judged out of proportion; the branch remains untested by this campaign.
