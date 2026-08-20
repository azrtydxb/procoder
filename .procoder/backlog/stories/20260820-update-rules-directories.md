# Update existing rules directories

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

All existing rules directories (`.cursor/rules/`, `.windsurf/rules/`, `.roo/rules/`, `.clinerules/`, `.kiro/steering/`, `.agents/rules/`) must be verified for consistency. They should all reference the same canonical content and not have conflicting instructions. Add version references so rules auto-update when the skill core is updated.

## Acceptance criteria

- [ ] All rule files (`.cursor/rules/procoder.mdc`, `.windsurf/rules/procoder.md`, `.roo/rules/procoder.md`, `.clinerules/procoder.md`, `.kiro/steering/procoder.md`, `.agents/rules/procoder.md`) exist
- [ ] All rule files contain identical content (or reference same source)
- [ ] No conflicting instructions across rule files
- [ ] Version references present so rules sync with skill updates
- [ ] No orphaned rule files from removed integrations

## Evidence

