# Create internal/releases/upgrade.go — Download & install

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Implement the download-and-install flow that fetches the latest procoder binary from GitHub releases and replaces the running binary.

## Acceptance criteria

- [x] `internal/releases/upgrade.go` created
- [x] `DownloadAndInstall(latest string, out func(string)) int` downloads and installs
- [x] Platform detection: use `GOOS/GOARCH` or `uname -s/-m` to determine dist directory (matches launcher.sh logic)
- [x] Asset URL constructed from release: find matching `procoder-<os>-<arch>` in release assets
- [x] Download to temp file via `os.CreateTemp`
- [x] Temp file is made executable: `os.Chmod +x`
- [x] Binary resolution: find the procoder binary from `os.Args[0]` or from the current working directory
- [x] Atomic replace: rename temp file over the original binary (or copy then rename)
- [x] Refuse to downgrade: compare current version before installing
- [x] If download fails: leave current binary untouched, clean up temp file
- [x] Interactive prompt: "Update procoder X.Y.Z → A.B.C? (y/N)"
- [x] No TTY: print warning, do not hang
- [x] Tests: mock HTTP server serves fake binary; download succeeds; temp file is cleaned up on failure

## Evidence

- internal/releases/upgrade.go: `Upgrade` resolves the running binary with symlinks followed (N-07), refuses to move backwards (N-06), asks before anything is downloaded (N-08), and renames last (N-04).
- The temp file is created beside the target, not in the system temp directory: a rename across filesystems fails, and the copy fallback would be the non-atomic write this exists to avoid. A deferred Remove clears the partial file on every failure path.
- The close before the rename is checked — a write that fails on close is a failed write, and renaming it would install the failure.
- TestUpgradeReplacesTheBinaryOnlyAfterAYes: new bytes on disk, executable bit set.
- TestAFailedDownloadLeavesTheWorkingBinary: a 500 mid-download leaves the old binary byte-identical and the output names what is still installed.
- TestADeclinedUpgradeChangesNothing: no download, no replacement, and no litter left beside the binary.
- DEVIATION: the plan named `uname` for the platform; runtime.GOOS/GOARCH is the same answer without a subprocess, and works on Windows where uname does not.
