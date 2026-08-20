# Procoder self-update — version check, warning, and upgrade

Status: done 2026-08-20
Created: 2026-08-20

## Goal

When procoder detects a newer version is available on GitHub releases, it warns the user and offers to upgrade. Users can upgrade via `procoder self-upgrade` or directly from the version-check prompt. The version check has a hard 1-second timeout, never blocks the AI coder, and refuses downgrades.
