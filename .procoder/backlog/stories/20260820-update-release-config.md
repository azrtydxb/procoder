# Update procoder release config with new version-bearing files

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Update `.procoder/config.toml` `[release] files` list to include all new version-bearing files created by this plan: `plugin.json`, `mcp.json`, `skills/procoder/SKILL.md`, `com.anthropic.claude-code/hooks/claude-hooks.json`, `vscode/package.json`, `cline/plugin.json`, `roo/plugin.json`, `gemini-extension.json`. This ensures `procoder release` syncs all versions across the new files.

## Acceptance criteria

- [ ] `[release] files` in config.toml updated with all new files
- [ ] List includes: `plugin.json`, `mcp.json`, `skills/procoder/SKILL.md`, all new plugin manifests
- [ ] Old files removed from list only if no longer version-bearing
- [ ] `procoder release --dry-run` (or equivalent) validates the file list
- [ ] No existing version-bearing files are removed from sync

## Evidence
