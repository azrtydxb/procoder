# Self-Update: Version Check & Upgrade

Status: open 2026-08-20
Created: 2026-08-20
Milestone: self-update
Spec: self-update

## Description

This epic adds version detection, upgrade, and self-update to procoder. It introduces the `releases` package (stdlib-only, no external dependencies) that queries GitHub releases API, compares semver versions, downloads the correct binary for the platform, and installs it atomically. The `version --check` flag warns when a newer version exists and offers upgrade. The `self-upgrade` command downloads and installs directly. The SessionStart hook runs a background version check and prints warnings to stderr. Downgrade protection is enforced.
