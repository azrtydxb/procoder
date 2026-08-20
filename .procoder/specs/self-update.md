# Spec: Self-Update — Version Check & Upgrade

Status: draft

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

- [R-01] `procoder version --check` queries GitHub releases API for `github.com/azrtydxb/procoder` and returns the latest tagged version
- [R-02] `procoder version --check` compares the current version against the latest tagged version (semver comparison, only patches/minors)
- [R-03] If a newer version exists, `--check` prints a warning: `== procoder: newer version X.Y.Z is available (current: A.B.C)`
- [R-04] `--check` asks the user interactively (TTY) if they want to upgrade, using the existing `copilot.Prompt` interaction pattern
- [R-05] If the user answers yes, `procoder self-upgrade` (or `--check --upgrade`) downloads and installs the new version
- [R-06] `procoder self-upgrade` is also a top-level command: `procoder self-upgrade` without checking first
- [R-07] The version check is integrated into the SessionStart hook output so the warning is visible at every session
- [R-08] When no TTY, the warning is still printed (in hook output) but no upgrade prompt is sent

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

- [ ] C-01: `procoder version --check` on latest version prints nothing (or "up to date") — verified by: Run on a box with the latest installed
- [ ] C-02: `procoder version --check` on old version prints warning + asks to upgrade — verified by: Run with an old dev binary
- [ ] C-03: `procoder self-upgrade` downloads and installs the latest binary on the PATH — verified by: Run on a clean env; verify version advances
- [ ] C-04: `procoder self-upgrade` refuses to downgrade — verified by: Pin to old version, run self-upgrade; must fail
- [ ] C-05: TTY check path — warning prints but no hanging prompt when no terminal — verified by: Run with ` | head`or`/dev/null` stdin; must not hang
- [ ] C-06: Network timeout (1s) does not block hook output — verified by: Simulate slow/no network; verify hook still completes
- [ ] C-07: `procoder version` without flags still just prints the version — verified by: Run `procoder version`; must output one line
- [ ] C-08: Version check does not block the gate — verified by: Hook pre-tool-use must not wait for version check

## Open questions

- [O-1] Should the version check only warn for patches (A.B.C → A.B.D) or also for minors (A.B.C → A.(B+1).C)?
- [O-2] Should `procoder self-upgrade` work offline? (No — requires network)
- [O-3] Should the version check be configurable? (e.g., `[version] check = "warn"` / "off" / "upgrade")
- [O-4] How is a package-manager install detected, so the command can step
  aside instead of overwriting a file Homebrew or apt believes it owns?
