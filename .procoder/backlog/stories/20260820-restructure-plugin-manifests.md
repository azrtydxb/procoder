# Restructure existing plugin manifests

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

Update all existing plugin manifest files (`.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, `.devin-plugin/plugin.json`, `gemini-extension.json`, `package.json`) to follow the Agent Plugins specification. Move hooks to extension namespaces, add missing metadata (`$schema`, version, author, description), and ensure consistency across all manifests.

## Acceptance criteria

- [ ] `.claude-plugin/plugin.json` has hooks under `extensions.com.anthropic.claude-code.hooks`
- [ ] `.claude-plugin/plugin.json` has `$schema` and full metadata
- [ ] `.codex-plugin/plugin.json` has hooks under extension namespace with `$schema`
- [ ] `.devin-plugin/plugin.json` has version and description
- [ ] `gemini-extension.json` has version, description, repository, license
- [ ] `package.json` has `$schema`, author, homepage, repository, keywords
- [ ] All manifests have consistent name, version, description, author, license
- [ ] All JSON files parse without errors

## Evidence
