# ci-built-binaries

Status: complete

## Problem

Procoder ships five per-platform binaries by committing them. `dist/` is
39MB, it has been rewritten 61 times, and `.git` is 690MB for a working
tree a fraction of that size. Every release adds another 39MB that cannot
be removed without rewriting history.

The binaries are built by hand, and in one day that failed twice. v3.0.0
was tagged with 2.0.1 binaries still committed — every manifest said
3.0.0, the gate was green, the suite was green, and the plugin would have
installed something that reported one version and behaved like another.
The corrected build then failed CI's reproducibility check, because
`dist/` had been built before a later source edit and CI rebuilds from the
commit. Both now have guards. Both were symptoms of the same thing: a
manual step in a release is a step that will be skipped or mis-ordered.

The machinery to do better already exists. `internal/releases` downloads a
release asset for this platform, verifies it against the published
`SHA256SUMS`, and installs it only then — that is `procoder self-upgrade`,
and it is tested. Nothing needs inventing; it needs moving earlier.

## Users

**Somebody installing the plugin.** Gets a marketplace clone and expects
the first session to work. They should not know or care that a binary was
fetched, except when it could not be.

**Somebody on a plane.** Has no network on first run. The session must
stay usable and must not silently pretend the gate is running.

**The maintainer.** Should never build a binary again. Tag, and CI does
the rest.

**This repository.** Stops carrying 39MB per release forever.

## In scope

- [S-1] `hooks/launcher.sh` fetches the binary for its platform when none
  is cached, verifies it against the release's `SHA256SUMS`, installs it
  atomically, and execs it — reading the version from
  `.claude-plugin/plugin.json` so the script carries no version of its own.
- [S-2] The steady-state path is unchanged: when the binary is already
  cached the launcher execs it directly, with no network call, no
  subprocess and no added latency.
- [S-3] A failure to fetch splits by invocation. An invocation is a hook
  when its first argument is `hook` OR any argument is `--hook`: those
  warn on stderr, write nothing to stdout and exit 0. Every other
  invocation refuses, names the reason, and exits non-zero. The `--hook`
  half is not a nicety — SessionStart is wired as `launcher.sh principles
--hook`, so a split that only recognised `hook <sub>` would refuse
  loudly at session start and break the very sessions it exists to
  protect.
- [S-4] Two processes racing on first run cannot produce a partial
  binary: the download lands on a temporary path and is renamed into
  place.
- [S-5] CI builds all five targets at the tag and publishes them with a
  `SHA256SUMS` it generated, instead of copying files committed by a
  person.
- [S-6] `dist/` leaves the working tree, and nothing binary is committed
  in its place.
- [S-7] The checks that existed to police committed binaries are removed
  with their subject: CI's reproducibility job, and the release
  controller's shipped-binary version check.
- [S-8] The changed contract is recorded — the launcher's own comment, the
  docs, and an ADR — because "marketplace install, no runtime, no
  network" was a stated property and is being spent.

## Out of scope

- Rewriting git history to reclaim the 690MB. Decided against: it breaks
  every clone, every fork, and the commit SHAs the changelog and ADRs
  cite, to reclaim space nobody pays for twice.
- Signing or attestation beyond the published checksums. Worth doing
  later; not what this change is about.
- Any change to `procoder self-upgrade`, which already downloads and
  verifies and is the proof this approach works.
- Supporting a private or air-gapped mirror of the release assets. A real
  need for some adopters, and a separate piece of work with its own
  configuration surface.

## Constraints

- **Nothing is ever built locally.** Not the binaries, not a bootstrap,
  not the checksums. If CI did not produce it, it does not ship.
- **The hot path stays hot.** The launcher runs on every session start,
  every Bash call and every write. Once cached, it must do no more work
  than it does today.
- **A hook must never break the session.** Whatever goes wrong, the host
  keeps working and the user can see what is not running.
- **No silent green.** A command that cannot find its binary must not
  exit 0. That is the failure this project exists to catch, and it would
  be sitting in the launcher.
- **Verify before executing.** A downloaded binary is not run until its
  sha256 matches the published manifest.
- **POSIX sh only.** The launcher runs under whatever `/bin/sh` is, and
  on Windows under MSYS, Cygwin or Git Bash.

## Interfaces

| Surface                              | Behaviour                                                                                                                                                                                                                                       |
| ------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `hooks/launcher.sh`                  | Resolves, fetches when absent, verifies, execs.                                                                                                                                                                                                 |
| `<plugin>/dist/<os>-<arch>/procoder` | Where a fetched binary is cached. Version-scoped by the plugin directory itself.                                                                                                                                                                |
| `PROCODER_BIN`                       | An absolute path to a binary to use instead. **Bypasses verification** — it is the caller's own file and the launcher checks nothing about it. Exists for a mirror, a bisect, and for the tests, which need to point the launcher at a fixture. |
| `PROCODER_NO_FETCH`                  | Set, the launcher never downloads; absence is reported as it would be offline. This is how S-2 is asserted: with a binary cached, sabotaging the fetch must change nothing.                                                                     |
| GitHub release assets                | `procoder-<os>-<arch>` and `SHA256SUMS`, both published by CI.                                                                                                                                                                                  |

## Data

Nothing is stored in the repository. A fetched binary is cached inside the
plugin's own install directory, which is already version-scoped, so it
needs no version arithmetic and is removed with the version that owns it.

## Edge cases

- **No network on first run.** A hook warns and allows the action; a
  command refuses. Both say the same reason.
- **The plugin directory is not writable, or the network is down.** The
  fetch cannot cache. A hook fires on every write and every Bash call, so
  retrying each time would put a failing network call on the hot path
  dozens of times a minute. The failure is recorded beside the cache path
  with a timestamp and not retried until it ages out; within that window
  the launcher reports the recorded reason without touching the network.
  The record is not silence — every invocation still says what is wrong.
- **Two hooks fire at once with no binary.** Both may download; the rename
  makes the loser harmless and neither observes a partial file.
- **A checksum that does not match.** The download is discarded and
  nothing is executed, and then the ordinary split applies: a hook warns
  and allows, a command refuses. An earlier draft made this refuse even
  for hooks, on the reasoning that an unverified binary is worse than no
  binary. The reasoning is right and the conclusion did not follow: what
  protects the user is not executing the file, which happens either way.
  Exiting non-zero from a hook adds no safety and takes the session with
  it.
- **`SHA256SUMS` fetched but the platform's line is missing.** Treated as
  a failed verification, never as a pass.
- **A release that does not exist for the version in `plugin.json`.**
  Reported with the version and the URL tried. The launcher never falls
  back to the newest release: installing a version the plugin does not
  declare is worse than installing nothing, and it is the silent kind —
  everything would appear to work while the binary and the manifest
  disagreed, which is the defect this whole change came from.

- **`.claude-plugin/plugin.json` missing or unparseable.** The same: no
  version, no fetch, reported. Guessing is what "latest" would be.

- **The window between a version bump reaching the default branch and CI
  publishing its release.** A clone taken in those minutes declares a
  version whose assets do not exist yet. Hooks degrade with the message
  naming the missing release, commands refuse, and the next run succeeds.
  Accepted rather than engineered around: the alternative is resolving
  "some earlier version", which is the fallback ruled out above.
- **`curl` and `wget` both absent.** Reported as the reason, with the
  manual download instruction.
- **A verified binary that will not execute** — wrong architecture for
  the kernel, a filesystem mounted noexec. Reported as a failure of the
  same kind, with the path, rather than surfacing as a bare shell error
  the host attributes to itself.

- **A partially downloaded file from an interrupted run.** Never
  observed: the temporary path is what is written and only a complete,
  verified file is renamed into place.

## Failure modes

- **The launcher becomes slow.** Guarded by S-2: the cached path does no
  network work at all, and a test asserts no fetch is attempted when the
  binary is present.
- **A hook exits non-zero and the host stops.** Guarded by S-3 and by
  tests that feed each hook a payload with fetching disabled.
- **A command exits 0 having done nothing.** The silent green. Guarded by
  S-3's other half.
- **An unverified binary is executed.** Guarded by discarding on mismatch
  and by a test that plants a corrupted download.
- **CI publishes nothing and the release looks fine.** The release job
  must fail when it has no assets to attach, rather than publishing an
  empty release that every installer then fails against.

## Acceptance criteria

- [ ] [S-1] With no cached binary and a reachable network, the launcher
      fetches the asset for its platform, verifies it against the
      release's `SHA256SUMS`, caches it, and execs it — asserted end to
      end against a stub server rather than the real GitHub.
- [ ] [S-1] The version fetched is the one in `.claude-plugin/plugin.json`,
      asserted by changing that file and observing the URL requested.
- [ ] [S-2] With the binary already cached, the launcher makes no network
      call: asserted by running it with fetching sabotaged and requiring
      success.
- [ ] [S-3] `launcher.sh hook post-tool-use` with no binary and no network
      writes nothing to stdout, writes the reason to stderr, and exits 0.
- [ ] [S-3] `launcher.sh check` in the same conditions exits non-zero and
      names the reason.
- [ ] [S-3] `launcher.sh principles --hook` with no binary and no network
      exits 0 and writes nothing to stdout — the SessionStart shape, which
      a split matching only `hook <sub>` would have broken.
- [ ] [S-4] A checksum mismatch under a hook exits 0 and executes nothing;
      the same mismatch under a command exits non-zero. Both leave no file
      at the cache path.
- [ ] [S-1] A `plugin.json` that is missing, unparseable, or naming a
      version with no published release produces a failure saying what was
      tried, and no request for any other version.
- [ ] [S-3] A second invocation inside the failure window makes no network
      call and still reports the reason, asserted by counting requests
      against a stub server.
- [ ] [S-4] A download that is interrupted leaves no file at the cache
      path, asserted by killing mid-write or by writing to the temporary
      path and checking the destination.
- [ ] [S-4] A binary whose sha256 does not match the manifest is not
      executed and is not left behind, for a hook as much as for a
      command.
- [ ] [S-5] The release job builds all five targets from the tagged source
      and attaches them with a `SHA256SUMS` generated in that job, and
      fails rather than publishing a release with no assets.
- [ ] [S-6] No file under `dist/` is tracked, and `git ls-files` reports
      no binary anywhere in the tree.
- [ ] [S-7] CI runs no reproducibility job, and `procoder release` no
      longer checks a shipped binary — with the tests that covered both
      removed rather than left asserting behaviour that is gone.
- [ ] [S-8] The launcher's comment, `docs/`, and a new ADR each state that
      the first run fetches over the network, and what happens when it
      cannot.

## Open questions

<!-- none — the four that mattered were decided and are recorded below -->

## Decisions

- **D-1: the launcher fetches, in shell, and nothing binary is
  committed.** A bootstrap stub was the first recommendation and was
  wrong: a stub is a build, and a build that happens anywhere but CI is
  the thing being removed. Both objections to shell were checked and both
  were false — `internal/portability/launcher_test.go` already executes
  the real launcher, and `scripts/build-dist.sh` already carries a
  portable `sha256()`.

- **D-2: warn and no-op for hooks, refuse for commands.** Both halves are
  required, and only one of them is obvious. A hook that fails hard breaks
  a session for a tool meant to help; a command that passes quietly is a
  silent green in the launcher every other check runs through.

- **D-3: the history is left alone.** The 690MB is spent. Rewriting it
  breaks every clone and fork and invalidates cited SHAs, to reclaim space
  nobody pays for twice. Stopping the growth is the whole win.

- **D-5: a hook is recognised by `hook` OR `--hook`.** Written down
  because the first draft matched only the first shape, and SessionStart
  is wired as `principles --hook`. As originally worded the criterion
  would have made the launcher refuse at session start on any machine
  that could not fetch — the split intended to keep sessions alive
  breaking them at the one moment it was written for.

- **D-6: a checksum mismatch is not special.** Not executing the file is
  the protection; the exit code is not. A hook that fails hard on
  mismatch adds no safety and costs the session.

- **D-7: a failed fetch is remembered, briefly.** The alternative on a
  path that fires on every write is a failing network call dozens of
  times a minute. Remembering is not silence: the reason is printed every
  time.

- **D-4: the reproducibility check is dropped with its subject.** It
  answered one question — do these committed binaries match the source
  they were committed beside — and there will be no committed binaries and
  no committer. What is spent deliberately is third-party verifiability:
  nobody outside CI will be able to rebuild and confirm the published
  bytes. That is the trust already extended to every other CI-published
  artifact.
