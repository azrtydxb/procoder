# Spec: Multi-Marketplace Strategy

Status: draft

## Problem

Procoder is an AI coding governance harness that currently integrates with 13+ coding tools through hand-rolled directory conventions (`.cursor/rules/`, `.windsurf/rules/`, etc.). Each tool has different marketplace/extension/plugin systems. Procoder's root `plugin.json` is minimal and does not follow any marketplace schema. Most integrations are per-project rules only — they are not distributable as installable plugins on any marketplace.

We need a unified plugin architecture that:

1. Lists Procoder on the marketplaces that have one
2. Follows the Agent Plugins specification (agent-plugins.org) as the portable core
3. Is installable and discoverable per-platform
4. Uses MCP, hooks, skills, and agents to provide the best experience on each platform

## Users

- A developer who finds procoder on the marketplace their editor already
  has, installs it there, and expects it to work without cloning a
  repository or placing a binary by hand.
- The maintainer, who submits and re-submits to each marketplace and needs
  one manifest set to keep in sync rather than thirteen hand-rolled ones.
- The agent inside each client, which reads whatever that client reads —
  rules, skills, hooks, or MCP tools — and should find the same contract
  behind all four.

## In scope

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

## Out of scope

- Marketplaces with no public submission process, and closed or
  invite-only tiers — [O-2] asks whether the Claude Code marketplace has
  one, and until it is answered nothing is promised there.
- Building or hosting a marketplace of our own.
- Changing what the harness does. This is distribution: the gate, the
  chain, and the domains are untouched by it.
- Private or enterprise marketplace instances behind an organisation's own
  registry.

## Constraints

- [N-01] Single binary architecture: one procoder binary serves CLI, MCP, and hooks
- [N-02] Graceful degradation: individual component failures must not break the plugin
- [N-03] Hooks must be lightweight with appropriate timeouts
- [N-04] All paths in hooks and MCP must use `./` relative paths and `${PLUGIN_ROOT}`/`${PLUGIN_DATA}` variables

## Interfaces

- `plugin.json` at the repository root — the Agent Plugins v1.0.0 manifest
  ([R-01], [R-02]).
- `mcp.json` at the plugin root — an MCP stdio server exposing procoder's
  tools ([R-03]).
- Client extension directories under reverse-domain namespaces, e.g.
  `com.anthropic.claude-code/`, `com.kiro/` ([R-04]).
- `skills/procoder/SKILL.md` — the Agent Skills manifest ([R-09]).
- A VS Code extension manifest (`package.json` with `engines.vscode`,
  `main`, `contributes`) for the VS Code Marketplace ([R-06]).
- A GitHub App and Action for the GitHub Marketplace ([R-07]).
- The existing `.xxx-plugin/plugin.json` and `.xxx/rules/` files, migrated
  rather than abandoned ([R-10]).

## Data

Every manifest above carries the plugin version, and they are already
pinned to `.claude-plugin/plugin.json` by the gate — a release where one
manifest is stale is a release where all of them are. The new manifests
join that same list rather than starting a second source of truth.

No user data, credentials, or repository content travels to any
marketplace: what is submitted is the manifest set and the documentation
in this repository.

## Edge cases

- A client reads none of the new structure and still expects its old
  `.xxx/rules/` file — [R-10] migrates rather than deletes, so both paths
  answer until the old one is provably unused.
- A marketplace rejects or delists a submission; the other twelve
  integrations must be unaffected.
- Two marketplaces disagree about a manifest field with the same name.
- A client ships its own procoder version through its marketplace while a
  binary from another channel is already on PATH.

## Failure modes

- A component fails to load in one client: [N-02] requires the rest of the
  plugin to keep working, never a broken session.
- A hook exceeds its budget: [N-03] requires a timeout, and a gate that did
  not finish is reported as NOT run rather than clean.
- A manifest drifts from the schema after a marketplace updates it:
  validation is [C-01]'s job, and it must fail loudly at release time
  rather than silently at install time.
- A submission stalls in review: the repository stays installable by hand,
  which is how it works today.

## Acceptance criteria

- [ ] C-01: `plugin.json` validates against the Agent Plugins v1.0.0 schema — verified by: Run: `claude plugin validate ./plugin.json` (or equivalent lint)
- [ ] C-02: All existing plugin directories have updated manifests — verified by: Glob all `.*/plugin.json` files and verify `$schema`, `description`, `author`, `version`
- [ ] C-03: `mcp.json` exists at plugin root with stdio server definition — verified by: Check file exists with `mcpServers` key
- [ ] C-04: Client-specific hooks exist under `com.anthropic.claude-code/` namespace — verified by: Check `.claude-plugin/` structure has extension namespace
- [ ] C-05: `skills/procoder/SKILL.md` has proper YAML frontmatter (name, description, license) — verified by: Check YAML parsing of frontmatter
- [ ] C-06: VS Code extension manifest exists with required fields — verified by: Verify `package.json` in `vscode/` has `engines.vscode`, `main`, `contributes`
- [ ] C-07: Procoder submissions have been made to all identified marketplaces — verified by: Evidence: submission confirmation URLs or PR merge links
- [ ] C-08: No marketplace integration is broken by the restructuring — verified by: Run existing `procoder check` and `procoder test` on the repo

## Open questions

- [O-1] Which version of the Agent Plugins specification is current? (Research indicates v1.0.0 at agent-plugins.org)
- [O-2] Does the Claude Code marketplace offer a "curated" tier beyond community that requires approval?
- [O-3] What is the timeline for each marketplace review cycle?
- [O-4] Which sections of this spec's Criteria table are release blockers
  and which are follow-ups? [C-07] cannot be met on our own schedule —
  it depends on other people's review queues.
