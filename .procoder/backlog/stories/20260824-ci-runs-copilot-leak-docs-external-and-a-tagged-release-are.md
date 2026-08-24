# The GitHub-dependent commands, against a throwaway public repository

Status: open
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

- [ ] `ci --runs`, `copilot-leak`, `docs --external` and a tagged release
      are run against a throwaway public repository, and each either
      passes or is reported with what it did.
- [ ] The fixture's CI completes at least one full run and its result is
      read back through `procoder ci --runs`, not through the web UI.
- [ ] A release is cut end to end and the published artifact is verified
      by download, the way this repository's own releases are.

## Evidence
