# Add `self-upgrade` command

Status: open 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Add a standalone `procoder self-upgrade` command that directly downloads and installs the latest version without prompting for a version check first.

## Acceptance criteria

- [ ] `"self-upgrade"` case added to the command switch in `cmd/procoder/main.go`
- [ ] `selfUpgradeCmd` calls `releases.Latest()` then `upgrade.DownloadAndInstall()`
- [ ] Prints progress: "Checking for updates..." → "Downloading..." → "Installing..." → "Updated to X.Y.Z"
- [ ] On failure: prints error and exits 0 (upgrade failure should not crash the binary)
- [ ] Usage text includes: `self-upgrade        download and install the latest procoder version`
- [ ] Placement: after the `version` command in the usage text block

## Evidence
