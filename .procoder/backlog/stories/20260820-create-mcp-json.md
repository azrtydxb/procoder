# Create mcp.json portable MCP server config

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Create a portable MCP server configuration file (`mcp.json`) at the plugin root. This file defines how AI coding tools connect to Procoder as an MCP server. It must use the Agent Plugins MCP schema with stdio transport pointing to the procoder binary.

## Acceptance criteria

- [ ] `mcp.json` exists at repo root
- [ ] `$schema` points to the MCP server schema URL
- [ ] `mcpServers` key exists with at least one server definition
- [ ] Server uses `type: "stdio"` transport
- [ ] `command` uses `./` relative path to procoder binary
- [ ] `--mode mcp` is included as first arg
- [ ] `cwd` reference uses `${PLUGIN_ROOT}` variable
- [ ] JSON parses without errors

## Evidence
