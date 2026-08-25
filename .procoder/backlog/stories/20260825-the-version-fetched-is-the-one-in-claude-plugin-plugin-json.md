# The launcher carries no version of its own

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

`hooks/launcher.sh` must work for 3.1.0 and 4.0.0 alike without being
touched. It reads the version from `.claude-plugin/plugin.json`, which sits
beside it in every marketplace clone.

This is what makes the staleness failure impossible rather than merely
guarded: there is no version stamped into anything, so nothing can go
stale against anything else. It is the property that killed the committed
binaries, kept.

## Acceptance criteria

- [x] The version fetched is the one in `.claude-plugin/plugin.json`,
      asserted by changing that file and observing the URL requested.

## Evidence

- `TestTheVersionFetchedIsTheOneTheManifestDeclares`: a manifest saying
  4.5.6 makes the launcher request `/v4.5.6/…` and run what that release
  serves. Nothing in the script names a version.
- This is what makes the staleness failure impossible rather than guarded:
  with no version stamped anywhere, nothing can go stale against anything.
- proved by: hard-coding a version in the URL — the request goes to the
  wrong release and the test sees a 404 instead of a run.
