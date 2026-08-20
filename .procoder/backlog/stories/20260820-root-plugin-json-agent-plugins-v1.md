# Update root plugin.json to Agent Plugins v1.0.0

Status: open 2026-08-20
Created: 2026-08-20
Epic: marketplace-strategy
Sprint: -

## Description

The current root `plugin.json` has only `{"name": "procoder"}`. It must be rewritten to follow the Agent Plugins v1.0.0 specification with all required fields, the canonical `$schema` URL, author metadata, and hooks moved to the client-specific extension namespace.

## Acceptance criteria

- [ ] `plugin.json` at repo root has `$schema` pointing to `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json`
- [ ] All required fields present: `name`, `description`, `author` (object with `name`, optional `email`/`url`), `version`
- [ ] Optional metadata present: `license`, `homepage`, `repository`, `keywords`
- [ ] Hook config moved to `extensions.com.anthropic.claude-code.hooks` per spec
- [ ] JSON parses without errors
- [ ] `procoder release` includes this file in sync verification

## Evidence

