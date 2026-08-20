# Spec: Multi-Marketplace Strategy

## Background

Procoder is an AI coding governance harness that currently integrates with 13+ coding tools through hand-rolled directory conventions (`.cursor/rules/`, `.windsurf/rules/`, etc.). Each tool has different marketplace/extension/plugin systems. Procoder's root `plugin.json` is minimal and does not follow any marketplace schema. Most integrations are per-project rules only — they are not distributable as installable plugins on any marketplace.

We need a unified plugin architecture that:

1. Lists Procoder on the marketplaces that have one
2. Follows the Agent Plugins specification (agent-plugins.org) as the portable core
3. Is installable and discoverable per-platform
4. Uses MCP, hooks, skills, and agents to provide the best experience on each platform

## Requirements

### Functional

- [R-01] Procoder must follow the Agent Plugins specification v1.0.0 as its portable plugin core
- [R-02] Procoder must publish a `plugin.json` manifest compliant with the Agent Plugins schema
- [R-03] Procoder must publish an `mcp.json` defining an MCP stdio server exposing procoder tools
- [R-04] Procoder must create client-specific extension directories under reverse-domain namespaces (e.g. `com.anthropic.claude-code/`, `com.kiro/`)
- [R-05] Procoder must submit to every marketplace that exists and has a public submission process
- [R-06] Procoder must submit to the VS Code Marketplace as a native extension (works in Cursor, Windsurf, Cline)
- [R-07] Procoder must register a GitHub App + Action for the GitHub Marketplace
- [R-08] Procoder must create formal plugin manifests for Cline and Roo Code
- [R-09] Procoder must create a `SKILL.md` compliant with the Agent Skills specification with proper frontmatter
- [R-10] Procoder must update all existing `.xxx-plugin/plugin.json` and `.xxx/rules/` files to the new structure

### Non-functional

- [N-01] Single binary architecture: one procoder binary serves CLI, MCP, and hooks
- [N-02] Graceful degradation: individual component failures must not break the plugin
- [N-03] Hooks must be lightweight with appropriate timeouts
- [N-04] All paths in hooks and MCP must use `./` relative paths and `${PLUGIN_ROOT}`/`${PLUGIN_DATA}` variables

## Open Questions

- [O-1] Which version of the Agent Plugins specification is current? (Research indicates v1.0.0 at agent-plugins.org)
- [O-2] Does the Claude Code marketplace offer a "curated" tier beyond community that requires approval?
- [O-3] What is the timeline for each marketplace review cycle?

## Criteria

| #    | Criterion                                                                           | Verification                                                                             |
| ---- | ----------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| C-01 | `plugin.json` validates against the Agent Plugins v1.0.0 schema                     | Run: `claude plugin validate ./plugin.json` (or equivalent lint)                         |
| C-02 | All existing plugin directories have updated manifests                              | Glob all `.*/plugin.json` files and verify `$schema`, `description`, `author`, `version` |
| C-03 | `mcp.json` exists at plugin root with stdio server definition                       | Check file exists with `mcpServers` key                                                  |
| C-04 | Client-specific hooks exist under `com.anthropic.claude-code/` namespace            | Check `.claude-plugin/` structure has extension namespace                                |
| C-05 | `skills/procoder/SKILL.md` has proper YAML frontmatter (name, description, license) | Check YAML parsing of frontmatter                                                        |
| C-06 | VS Code extension manifest exists with required fields                              | Verify `package.json` in `vscode/` has `engines.vscode`, `main`, `contributes`           |
| C-07 | Procoder submissions have been made to all identified marketplaces                  | Evidence: submission confirmation URLs or PR merge links                                 |
| C-08 | No marketplace integration is broken by the restructuring                           | Run existing `procoder check` and `procoder test` on the repo                            |
