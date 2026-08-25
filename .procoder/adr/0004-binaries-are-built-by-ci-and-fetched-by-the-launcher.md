# 0004 — binaries are built by CI and fetched by the launcher

Status: accepted
Date: 2026-08-25

## Context

Procoder shipped five per-platform binaries by committing them to `dist/`.
The plugin is a marketplace clone and `hooks/launcher.sh` exec'd the binary
for the host directly, which made the install offline, immediate, and free
of any runtime. The launcher's own comment stated it as a contract:
"marketplace install, no runtime, no network."

That contract had a price that grew. `dist/` is 39MB and had been rewritten
61 times, so `.git` reached 690MB for a working tree a fraction of that
size, and every release added another 39MB that could not be removed
without rewriting history.

It also had a price that arrived all at once. In a single day, v3.0.0 was
tagged with 2.0.1 binaries still committed — every manifest read 3.0.0, the
gate was green, the suite was green, and the plugin would have installed
something that reported one version and behaved like another. The corrected
build then failed CI's reproducibility check, because `dist/` had been built
before a later source edit and CI rebuilds from the commit. Both got guards.
Both were the same thing underneath: a manual step in a release is a step
that will eventually be skipped or mis-ordered.

The constraint that settled the design was stated plainly by the
maintainer: nothing is ever built locally. Not the binaries, not a
bootstrap, not the checksums.

## Decision

CI builds all five targets at the tag and publishes them with a
`SHA256SUMS` it generated. `hooks/launcher.sh` fetches the one binary this
platform needs on first use, verifies it against that manifest, caches it
beside the plugin, and execs it. Nothing binary is committed.

**Why the launcher and not a bootstrap binary.** The first design was a
small committed Go stub that did the fetching, so verification could live
in the tested `internal/releases` code. A stub is a build, and a build that
happens anywhere but CI is the thing being removed. What remains is the
only artifact that needs no build at all: a shell script, written rather
than compiled and reviewed as text.

**Why shell was not the risk it looked like.** Two objections were raised
and both were false on inspection.
`internal/portability/launcher_test.go` already executed the real launcher
against a throwaway plugin root, so the harness for testing it by running
it existed. And `scripts/build-dist.sh` already carried a portable
`sha256()` using `sha256sum` where present and `shasum -a 256` otherwise.

**Why not CI committing `dist/` back.** It would have fixed the manual step
and kept the offline guarantee, and left the 39MB-per-release growth
exactly where it was.

**Why not a plugin install hook.** The manifest declares `hooks` for
lifecycle events and nothing resembling a post-install step was found.
Ruled out as unverified rather than assumed absent.

## Consequences

**Easier.** No human builds a release binary again. The repository stops
growing by 39MB a release. The staleness failure becomes impossible rather
than guarded, because the launcher reads the version from
`.claude-plugin/plugin.json` and carries none of its own. The test job
takes a shallow checkout now that nothing needs history.

**Harder, and paid deliberately.**

_A first run needs the network._ The offline guarantee is spent. What
protects the session is the split: an invocation that is `hook <sub>` or
carries `--hook` warns on stderr, writes nothing to stdout and exits 0 —
no stdout being "no decision" to PreToolUse and "no context" to
PostToolUse — while every other invocation refuses and exits non-zero. A
command that exited 0 having run nothing would be a silent green
underneath every check in the tool.

_Third-party verifiability is gone._ CI's reproducibility job rebuilt the
committed binaries from the commit that carried them and compared digests,
which let anyone confirm the shipped bytes came from the shipped source.
With nothing committed there is nothing to rebuild against, and the
provenance is the workflow run. This is the trust already extended to every
other CI-published artifact, and it is recorded here rather than left to be
discovered.

_A release with no assets breaks every install, not merely upgrades._ The
release job therefore fails rather than publishing one, and checks the
manifest has five lines before uploading.

_A window exists between a version bump reaching the default branch and CI
publishing its release._ A clone taken in those minutes declares a version
whose assets do not exist yet; hooks degrade with a message naming the
missing release and commands refuse. Accepted rather than engineered
around, because the alternative — resolving "some earlier version" — would
install something the plugin does not declare, which is the failure this
decision exists to remove.
