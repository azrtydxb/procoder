# Create Cline and Roo Code plugin manifests

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Create formal plugin manifests for Cline and Roo Code under `cline/` and `roo/` directories. Both use the Agent Plugins specification with the same portable core format. Each manifest defines the plugin name, version, description, MCP server config, and skills path.

## Acceptance criteria

- [ ] `cline/plugin.json` created with Agent Plugins format
- [ ] `roo/plugin.json` created with Agent Plugins format
- [ ] Both have: name, version, description, author, license, keywords
- [ ] Both include MCP server config with stdio transport
- [ ] Both reference skills path (`skills/procoder/SKILL.md`)
- [ ] Both parse as valid JSON
- [ ] JSON files have consistent metadata (matching other manifests)

## Evidence
