# Spec: Self-Update — Version Check & Upgrade

## Background

Procoder's binary version is stamped at build time (`cmd/procoder/main.go` line 170: `var version = "dev"` → `-ldflags -X ...` sets it to the release version). The `procoder version` command prints this version. But nothing warns the user when a newer version exists, and there is no way to upgrade without manually downloading from GitHub or installing via a package manager.

AI coders run procoder via hooks at every session start. If they are running an older version, they may not know about bug fixes, new features, or security patches. Users never see a "new version available" warning and have no path to upgrade.

## Requirements

### Functional

- [R-01] `procoder version --check` queries GitHub releases API for `github.com/azrtydxb/procoder` and returns the latest tagged version
- [R-02] `procoder version --check` compares the current version against the latest tagged version (semver comparison, only patches/minors)
- [R-03] If a newer version exists, `--check` prints a warning: `== procoder: newer version X.Y.Z is available (current: A.B.C)`
- [R-04] `--check` asks the user interactively (TTY) if they want to upgrade, using the existing `copilot.Prompt` interaction pattern
- [R-05] If the user answers yes, `procoder self-upgrade` (or `--check --upgrade`) downloads and installs the new version
- [R-06] `procoder self-upgrade` is also a top-level command: `procoder self-upgrade` without checking first
- [R-07] The version check is integrated into the SessionStart hook output so the warning is visible at every session
- [R-08] When no TTY, the warning is still printed (in hook output) but no upgrade prompt is sent

### Non-functional

- [N-01] Version check uses only stdlib (`net/http`) to query GitHub releases API — no new dependencies
- [N-02] The version check has a hard timeout (1 second) — a slow network must not block the session
- [N-03] The version check must not block the gate — it runs in a goroutine and the gate proceeds regardless
- [N-04] Upgrade must be atomic: download to temp file, verify checksum if available, then rename
- [N-05] `procoder version` should continue to just print the version (unchanged)

### Security

- [N-06] Downgrade protection: `procoder self-upgrade` refuses to install a version older than the current one
- [N-07] The binary path for `procoder self-upgrade` resolves the binary from PATH (same way hooks resolve it)
- [N-08] The downloaded binary is not executed until the user confirms (or the `--force` flag is set)

## Open Questions

- [O-1] Should the version check only warn for patches (A.B.C → A.B.D) or also for minors (A.B.C → A.(B+1).C)?
- [O-2] Should `procoder self-upgrade` work offline? (No — requires network)
- [O-3] Should the version check be configurable? (e.g., `[version] check = "warn"` / "off" / "upgrade")

## Criteria

| #    | Criterion                                                                     | Verification                                          |
| ---- | ----------------------------------------------------------------------------- | ----------------------------------------------------- |
| C-01 | `procoder version --check` on latest version prints nothing (or "up to date") | Run on a box with the latest installed                |
| C-02 | `procoder version --check` on old version prints warning + asks to upgrade    | Run with an old dev binary                            |
| C-03 | `procoder self-upgrade` downloads and installs the latest binary on the PATH  | Run on a clean env; verify version advances           |
| C-04 | `procoder self-upgrade` refuses to downgrade                                  | Pin to old version, run self-upgrade; must fail       |
| C-05 | TTY check path — warning prints but no hanging prompt when no terminal        | Run with `                                            | head`or`/dev/null` stdin; must not hang |
| C-06 | Network timeout (1s) does not block hook output                               | Simulate slow/no network; verify hook still completes |
| C-07 | `procoder version` without flags still just prints the version                | Run `procoder version`; must output one line          |
| C-08 | Version check does not block the gate                                         | Hook pre-tool-use must not wait for version check     |
