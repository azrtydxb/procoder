# The fetch: get it, verify it, cache it, run it

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 019-the-launcher-fetches-verifies-and-caches-its-own-binary

## Description

The core of the change. With nothing cached, the launcher reads the
version from `.claude-plugin/plugin.json`, downloads the asset for this
platform and the release's `SHA256SUMS`, checks one against the other,
installs the binary, and execs it with everything passed through.

Verification is not optional and not deferred: a file that has not been
checked is never executed. `scripts/build-dist.sh` already carries a
portable `sha256()` using `sha256sum` where it exists and `shasum -a 256`
otherwise, and that is the shape to reuse rather than reinvent.

Tested against a stub server rather than the real GitHub, so the test
answers about the launcher and not about the network.

## Acceptance criteria

- [ ] With no cached binary and a reachable network, the launcher fetches
      the asset for its platform, verifies it against the release's
      `SHA256SUMS`, caches it, and execs it — asserted end to end against
      a stub server rather than the real GitHub.
- [ ] The verification uses the repository's existing portable sha256
      approach, and a mismatch is detectable by the test rather than
      merely assumed to work.

## Evidence
