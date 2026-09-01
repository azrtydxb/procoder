# Spec: Self-Update — Version Check & Upgrade

Status: complete

## Problem

Procoder's binary version is stamped at build time (`cmd/procoder/main.go` line 170: `var version = "dev"` → `-ldflags -X ...` sets it to the release version). The `procoder version` command prints this version. But nothing warns the user when a newer version exists, and there is no way to upgrade without manually downloading from GitHub or installing via a package manager.

AI coders run procoder via hooks at every session start. If they are running an older version, they may not know about bug fixes, new features, or security patches. Users never see a "new version available" warning and have no path to upgrade.

## Users

- The developer running an old binary without knowing it, who finds out
  when a bug that was fixed weeks ago bites them again.
- The AI coder, which runs procoder through hooks at every session start
  and cannot tell whether the verdicts it is acting on come from a current
  binary.
- The maintainer, whose bug fix is only real once it is on other people's
  machines — a release nobody installs has changed nothing.

## In scope

- [S-1] `procoder version --check` queries GitHub releases API for `github.com/azrtydxb/procoder` and returns the latest tagged version
- [S-2] `procoder version --check` compares the current version against the latest tagged version (semver comparison, only patches/minors)
- [S-3] If a newer version exists, `--check` prints a warning: `== procoder: newer version X.Y.Z is available (current: A.B.C)`
- [S-4] `--check` asks the user interactively (TTY) if they want to upgrade, using the existing `copilot.Prompt` interaction pattern
- [S-5] If the user answers yes, `procoder self-upgrade` (or `--check --upgrade`) downloads and installs the new version
- [S-6] `procoder self-upgrade` is also a top-level command: `procoder self-upgrade` without checking first
- [S-7] The version check is integrated into the SessionStart hook output so the warning is visible at every session
- [S-8] When no TTY, the warning is still printed (in hook output) but no upgrade prompt is sent

## Out of scope

- Package-manager installs. Where a binary came from Homebrew or a
  distribution package, that manager owns the upgrade and this command must
  not fight it — [O-4] asks how that is detected.
- Downgrading, and installing a chosen version: [N-06] refuses to move
  backwards at all.
- Automatic upgrades without consent. Every path here ends in a question,
  and [N-08] keeps the downloaded binary unexecuted until it is answered.
- Updating anything other than the binary — plugin manifests, rules files
  and skills travel with the repository, not with this command.

## Constraints

- [N-01] Version check uses only stdlib (`net/http`) to query GitHub releases API — no new dependencies
- [N-02] The version check has a hard timeout (1 second) — a slow network must not block the session
- [N-03] The version check must not block the gate — it runs in a goroutine and the gate proceeds regardless
- [N-04] Upgrade must be atomic: download to temp file, verify checksum if available, then rename
- [N-05] `procoder version` should continue to just print the version (unchanged)

### Security

- [N-06] Downgrade protection: `procoder self-upgrade` refuses to install a version older than the current one
- [N-07] The binary path for `procoder self-upgrade` resolves the binary from PATH (same way hooks resolve it)
- [N-08] The downloaded binary is not executed until the user confirms (or the `--force` flag is set)

## Interfaces

- `procoder version --check` — queries, compares, warns, and offers
  ([R-01] to [R-04]).
- `procoder self-upgrade` — the same install path without checking first
  ([R-05], [R-06]).
- `procoder version` with no flags, unchanged: one line, the version
  ([N-05]).
- The SessionStart hook's output, which carries the warning where the coder
  and the user both see it ([R-07], [R-08]).
- GitHub's releases API for `github.com/azrtydxb/procoder`, over stdlib
  `net/http` only ([N-01]).

## Data

Nothing is stored: the check reads the current binary's stamped version and
GitHub's latest tag, and keeps neither. No telemetry leaves the machine —
the request asks GitHub what its newest release is and says nothing about
who is asking.

The upgrade writes one file: the new binary, downloaded to a temporary path
and renamed over the old one only after it is complete ([N-04]).

## Edge cases

- The binary is not writable — a system-wide install, or one owned by a
  package manager. The upgrade must say so rather than fail halfway.
- The running binary is the one being replaced.
- A build with no version stamped (`dev`), where there is nothing to
  compare.
- A release published with no asset for this platform.
- Two procoder binaries on PATH, and the upgrade replaces the wrong one —
  [N-07] resolves it the way hooks do, which is the only definition that
  matches what actually runs.
- The newest release is older than the running build, which happens to
  anyone working on an unreleased branch.

## Failure modes

- The network is slow or absent: [N-02] caps the wait at one second and
  [N-03] keeps it off the gate's path. A check that did not answer is
  reported as not done, never as up to date.
- GitHub answers with something unexpected: the check says it could not
  determine the latest version, and no upgrade is offered on a guess.
- The download is truncated or corrupt: [N-04] makes the rename the last
  step, so a failed download leaves the working binary in place.
- The upgrade succeeds and the new binary does not run: the user is left
  with a broken tool, which is why [N-08] wants the confirmation and why a
  verified checksum matters more than speed.

## Acceptance criteria

- [x] [S-1] [S-2] C-01: `procoder version --check` on latest version prints nothing (or "up to date") — Run: `procoder version --check` on a box with the latest installed; verified live this session, when 3.5.0 answered "nothing to do".
- [x] [S-2] [S-3] [S-4] C-02: `procoder version --check` on old version prints warning + asks to upgrade — `TestShouldWarnOnEveryNewerRelease` in `internal/releases` asserts the warning, `TestUpgradeReplacesTheBinaryOnlyAfterAYes` the prompt.
- [x] [S-5] [S-6] C-03: `procoder self-upgrade` downloads and installs the latest binary on the PATH — `TestUpgradeReplacesTheBinaryOnlyAfterAYes` in `internal/releases`.
- [x] [S-5] C-04: `procoder self-upgrade` refuses to downgrade — `TestUpgradeRefusesToGoBackwardsAndSaysWhenCurrent` in `internal/releases`.
- [x] [S-8] C-05: TTY check path — warning prints but no hanging prompt when no terminal — the no-TTY route in `internal/releases/upgrade.go`; fails if a pipe ever blocks on a prompt.
- [x] [S-7] C-06: Network timeout (1s) does not block hook output — `TestTheTimeoutIsEnforced` in `internal/releases`.
- [x] [S-1] C-07: `procoder version` without flags still just prints the version — Run: `procoder version`; exits 0 and one line.
- [ ] [S-7] C-08: Version check does not block the gate — the hook pre-tool-use must not wait for a version check; fails if the gate ever waits on a network call.

## Open questions

<!-- none — decisions recorded below -->

## Decisions

- [D-1] The check warns for any newer release — patch, minor and major.
  A major is exactly the change a user most needs to hear about, and
  suppressing it to avoid noise hides the one upgrade that changes
  behaviour. ([O-1] resolved.)
- [D-2] `[version] check = "warn" | "off"` in `.procoder/config.toml`,
  default `warn`, following D-OVERRIDE like every other domain policy. CI
  and scripted runs set `off`; nothing else is configurable, because a
  third value ("upgrade", meaning install without asking) would break the
  consent rule this feature is built around. ([O-3] resolved.)
- [D-3] The upgrade steps aside from package managers. The binary is
  resolved the way hooks resolve it, and when its real path (symlinks
  followed) sits under a manager's prefix — a Homebrew cellar or opt
  directory, /usr/bin, /usr/local/bin owned by root, a scoop shims
  directory — the command refuses and prints that manager's own upgrade
  command. Overwriting a file a package database believes it owns leaves
  the user with a version their manager will silently revert. The
  detection is a heuristic and is documented as one: it errs toward
  refusing, and `--force` is the escape hatch for a user who knows better.
  ([O-4] resolved.)
- [D-4] The upgrade requires the network and says so when it is absent; no
  offline path exists, because there is nothing to install from. ([O-2]
  resolved.)
