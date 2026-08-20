# Create VS Code extension scaffold

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Create a VS Code extension scaffold under `vscode/` that enables Procoder to be listed on the VS Code Marketplace. The extension will work natively in Cursor, Windsurf, Cline, and any other VS Code-based coding tool. It declares procoder commands, MCP server, configuration options, and uses the VS Code AI extensibility API.

## Acceptance criteria

- [ ] `vscode/package.json` created with all required VS Code fields
- [ ] Has `name`, `displayName`, `version`, `publisher`, `description`
- [ ] Has `engines.vscode` with minimum version
- [ ] Has `main` pointing to compiled output
- [ ] Has `categories` (at least `["Other"]` or `["Extension Packs"]`)
- [ ] Has `keywords` array
- [ ] Has `contributes.commands` with procoder subcommands
- [ ] Has `contributes.mcp.servers` declaring procoder MCP server
- [ ] Has `contributes.configuration` with settings
- [ ] Has `activationEvents` with `onStartupFinished`
- [ ] Has `galleryBanner`, badges, icon reference
- [ ] Has `sponsor` link to GitHub Sponsors
- [ ] `vscode/src/extension.ts` has stub `activate`/`deactivate` exports
- [ ] `vscode/tsconfig.json` present
- [ ] `vscode/README.md` present with marketplace description
- [ ] All JSON files parse without errors

## Evidence

