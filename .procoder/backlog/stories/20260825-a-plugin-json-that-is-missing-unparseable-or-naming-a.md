# No version, no guess

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: -

## Description

If `plugin.json` is missing, unparseable, or names a version with no
published release, the launcher says so and fetches nothing.

It never falls back to the newest release. Installing a version the plugin
does not declare is worse than installing nothing and it is the silent
kind: everything would appear to work while the binary and the manifest
disagreed, which is precisely the defect this whole epic came from.

The message names what was tried — the version, the URL — so the reader
can tell a typo from an outage from a release that has not published
yet.

## Acceptance criteria

- [ ] A `plugin.json` that is missing, unparseable, or naming a version
      with no published release produces a failure saying what was tried,
      and no request for any other version.
- [ ] The three causes are distinguishable in the message.

## Evidence
