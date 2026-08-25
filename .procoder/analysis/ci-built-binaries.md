# ci-built-binaries

## Question

Can the per-platform binaries stop being committed, and be fetched from
the release CI publishes instead — without breaking the hooks that run on
every session start, every Bash call and every write?

## What we know

**The cost of committing them is measured and compounding.** `dist/` is
39MB. `.git` is 690MB for a working tree a fraction of that size, because
`dist/` has been rewritten 61 times and every copy is permanent. Each
release adds roughly another 39MB that can never be removed without
rewriting history.

**Building them by hand has failed twice in one day.** v3.0.0 was tagged
with 2.0.1 binaries still committed — every manifest said 3.0.0, the gate
was green, the suite was green. Then the corrected build failed CI's
reproducibility check, because `dist/` had been built before a later
source edit and CI rebuilds from the commit. Both are now guarded, but
both are symptoms: a manual step in a release is a step that will be
skipped or mis-ordered.

**The download-and-verify path already exists and is tested.**
`internal/releases` carries `Latest`, `AssetName`, `Upgrade` and
`verify.go`: it asks GitHub for a release, downloads the asset for this
platform, verifies it against the published `SHA256SUMS`, and installs it
only then. That is `procoder self-upgrade`. Nothing here needs inventing;
it needs moving earlier in the lifecycle.

**The launcher is a hot path with no error budget.** `hooks/launcher.sh`
is what `claude-hooks.json` names for SessionStart, PreToolUse (Bash),
PostToolUse (Write|Edit), Stop and PreCompact. It resolves
`dist/$os-$arch/procoder` and execs it. A hook that fails takes the host's
session with it.

**The current design contract is explicit about the network.** The
launcher's own comment: "marketplace install, no runtime, no network."
Fetching at first use spends that property deliberately.

**The plugin's install directory is already version-scoped.**
`~/.claude/plugins/cache/procoder/procoder/3.0.0` — so a binary cached
beside the plugin needs no version arithmetic and disappears with the
version that owns it.

## What we do not know

- Whether Claude Code supports any post-install step for a plugin. The
  manifest declares `name`, `version`, `description`, `author`,
  `license` and `hooks`, and nothing else was found. Ruled out of this
  design rather than assumed absent.
- Whether the plugin cache directory is reliably writable on every host
  and platform. The design must degrade rather than assume.
- How two hooks firing at once behave when both find no binary. Concurrent
  first-run downloads need an answer, not a hope.

## Options

Four were weighed and the decision is recorded below.

1. **A shell launcher that downloads.** Lightest repository — nothing
   binary committed at all. Puts checksum verification, the
   security-critical part, in POSIX `sh` rather than the tested Go path.
2. **A committed bootstrap stub that downloads.** Verification stays in
   Go. Still commits five files, but ones that change rarely rather than
   every release, so the history stops growing.
3. **CI builds and commits `dist/` back.** Fixes the manual step and keeps
   the no-network guarantee, and does nothing about 39MB per release.
4. **A plugin install hook.** Cleanest if it exists; unverified.

## The constraint that settled it

Nothing is ever built locally. Not the binaries, not a bootstrap, not the
checksums. CI builds and CI publishes, or it does not happen.

That rules out the Go stub unless CI commits it back, and it rules out
computing checksums on a maintainer's machine. What remains is the only
artifact that needs no build at all: a shell script, which is written
rather than compiled and reviewed as text in a pull request.

Two objections raised against shell earlier turned out to be wrong:

- **"Verification in sh is untestable."** `internal/portability/launcher_test.go`
  already executes the real `hooks/launcher.sh` against a throwaway plugin
  root whose fake binaries print the path they were exec'd as. The harness
  for testing this script by running it exists.
- **"sha256 in sh is not portable."** `scripts/build-dist.sh` already
  carries a `sha256()` that uses `sha256sum` where it exists and `shasum
-a 256` otherwise. The problem is solved in this repository.

No other language is needed for a universal artifact. The job is: read a
version, fetch two files, compare a hash, exec. Every platform procoder
targets ships a POSIX shell, Windows included through the MSYS, Cygwin
and Git Bash arms the launcher already handles.

## Recommendation

**Option 2, with the stub kept off the steady-state path.**

The launcher looks for the real binary first and execs it directly, as it
does today; the stub runs only when that binary is absent. After the first
run the hot path is byte-for-byte the behaviour that exists now, which
matters when the path in question fires on every write.

Three consequences follow from the decisions taken, and the third is the
one worth stating loudest:

- **Offline is a warning, not a failure — for hooks.** A hook that cannot
  fetch prints why on stderr, writes nothing to stdout, and exits 0. No
  stdout is "no decision" to PreToolUse and "no context" to PostToolUse,
  so the session stays usable and the user can see the gate is not
  running.

- **Offline is a failure — for commands.** `procoder check` that cannot
  find its binary must NOT exit 0. That would be a silent green in the
  command that exists to prevent them, and it is the same shape as every
  defect this release fixed. The launcher therefore branches on whether
  it was invoked as `hook`: no-op safely there, refuse loudly everywhere
  else.

- **CI publishing `SHA256SUMS` is strictly better than today**, where the
  maintainer's machine is the source of truth for what the world
  downloads. It also changes what the existing reproducibility check is
  for: it stops proving a committer's binaries match their source, and
  starts proving CI's build is reproducible from the tag.

Concurrency is answered by writing to a temporary file and renaming into
place, so a second process either sees no binary or sees a complete one,
never a half-written one.

## Decisions

- **D-1: a committed Go bootstrap stub, not shell.** The verification is
  the point of the exercise, and it already exists, tested, in
  `internal/releases`. Rewriting sha256 verification in POSIX `sh` to save
  five files would move the one security-critical step into the least
  testable language in the repository.
- **D-2: warn and no-op for hooks, refuse for commands.** Both halves are
  required. A hook that fails hard breaks the session for a tool that is
  meant to help; a command that passes quietly is the exact failure this
  project is built to catch.
- **D-3: stop the growth, leave the history.** The 690MB is already
  spent. Rewriting it breaks every clone, every fork, and the commit
  SHAs the changelog and the ADRs cite, to reclaim space nobody is
  paying for twice.
- **D-4: CI publishes the checksums.** If CI builds the binaries, no
  human machine is the source of truth for what anyone downloads.
- **D-5: the launcher does the work, in shell, and nothing binary is
  committed.** Supersedes the bootstrap stub of D-1. A stub is a build,
  and a build that happens anywhere but CI is the thing being removed. The
  two objections to shell were checked and both were false.
- **D-6: the reproducibility check is dropped rather than retargeted.**
  It existed to answer one question — do these committed binaries match
  the source they were committed beside — and there will be no committed
  binaries and no committer. What it also gave, and what is being spent
  deliberately, is third-party verifiability: with nothing to rebuild
  against, nobody outside CI can independently confirm the published bytes
  came from the tagged source. That is the trust already extended to every
  other CI-published artifact, and it is recorded here as a choice rather
  than left to be discovered as an omission.
