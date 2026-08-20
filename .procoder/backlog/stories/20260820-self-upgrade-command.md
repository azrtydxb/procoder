# Add `self-upgrade` command

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Add a standalone `procoder self-upgrade` command that directly downloads and installs the latest version without prompting for a version check first.

## Acceptance criteria

- [x] `"self-upgrade"` case added to the command switch in `cmd/procoder/main.go`
- [x] `selfUpgradeCmd` calls `releases.Latest()` then `upgrade.DownloadAndInstall()`
- [x] Prints progress: "Checking for updates..." → "Downloading..." → "Installing..." → "Updated to X.Y.Z"
- [x] On failure: prints error and exits 0 (upgrade failure should not crash the binary)
- [x] Usage text includes: `self-upgrade        download and install the latest procoder version`
- [x] Placement: after the `version` command in the usage text block

## Evidence

- `procoder self-upgrade [--force]` is a top-level command, dispatched in cmd/procoder/main.go and listed in the usage text and docs.Commands.
- D-3: where the resolved binary sits under a package manager's prefix, it refuses and prints that manager's own command — brew upgrade, snap refresh, nix profile upgrade, scoop update, choco upgrade, or the distribution's manager for /usr/bin. TestAPackageManagerBinaryIsRefusedWithItsOwnCommand covers each, and asserts a hand-installed path is the user's to replace.
- Symlinks are resolved first: a Homebrew install is usually a link from /usr/local/bin into the cellar, and judging the link would miss every one.
- The heuristic is marked with the debt marker naming its revisit condition, and --force is the documented way past it.
- --force skips only the manager refusal, never the consent (N-08).
