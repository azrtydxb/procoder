# ci-built-binaries

Status: done 2026-08-25
Created: 2026-08-25
Spec: ci-built-binaries @ 78dbaa5ae1d9

## Description

Procoder ships five per-platform binaries by committing them, and it has
cost 690MB of git history for a working tree a fraction of that size —
`dist/` is 39MB and has been rewritten 61 times, permanently, every one.

It has also cost two failures in a single day. v3.0.0 was tagged with
2.0.1 binaries still committed, with every manifest, the gate and the
suite green. The corrected build then failed CI's reproducibility check
because `dist/` was built before a later source edit. Both are symptoms of
one thing: a manual step in a release is a step that will be skipped or
mis-ordered.

This epic removes the step rather than guarding it again. Nothing is built
locally, ever. CI builds all five at the tag and publishes them with
checksums it generated, and `hooks/launcher.sh` fetches the one binary
this machine needs on first use, verifies it, and caches it beside the
plugin.

The launcher is written rather than compiled, which is the whole point: it
is the only artifact that needs no build at all. Two objections to putting
verification in shell were checked and both were false — the launcher is
already executed by `internal/portability/launcher_test.go`, and a
portable `sha256()` already exists in `scripts/build-dist.sh`.

The steady-state path does not change. Once cached, the launcher execs the
binary exactly as it does today, on a path that fires on every session
start, every Bash call and every write.
