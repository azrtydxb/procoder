# Plan: Multi-Marketplace Strategy

## Context

Procoder currently has 13+ hand-rolled integrations across AI coding tools. Each integration is scattered across directories (`.cursor/rules/`, `.windsurf/rules/`, etc.) with minimal manifest files. No integration follows the Agent Plugins specification. No marketplace submissions exist.

The plan converts all integrations into a unified plugin architecture based on the Agent Plugins v1.0.0 specification, creates formal marketplace submissions, and improves hooks/MCP/skills for every platform.

## Architecture Decision

Adopt a single portable core (`plugin.json`, `mcp.json`, `skills/`) with client-specific extension overlays under reverse-domain namespaces. One procoder binary serves CLI (`procoder <cmd>`), MCP stdio server (`procoder --mode mcp`), and hook launcher (`launcher.sh procoder ...`).

```
.
├── plugin.json              ← Agent Plugins v1.0.0 manifest (new, replaces root plugin.json)
├── mcp.json                 ← MCP server config (new)
├── skills/
│   └── procoder/
│       └── SKILL.md         ← Updated with Agent Skills frontmatter
├── com.anthropic.claude-code/  ← Claude Code hooks + config
│   ├── hooks/
│   │   └── claude-hooks.json
│   └── mcp.json
├── com.kiro/                    ← Kiro hooks + steering
│   └── hooks/
│       └── kiro-hooks.json
├── com.cursor/                  ← Cursor hooks
│   └── hooks/
│       └── cursor-hooks.json
├── vscode/                      ← VS Code extension (new directory)
│   ├── package.json             ← VS Code manifest
│   └── src/                     ← Extension source (stub)
├── cline/                       ← Cline plugin manifest (new)
│   └── plugin.json
├── roo/                         ← Roo Code plugin manifest (new)
│   └── plugin.json
├── .claude-plugin/              ← Legacy, restructured under extension namespace
├── .codex-plugin/               ← Updated manifest
├── .devin-plugin/               ← Updated manifest
├── .qoder/                      ← Updated plugin.json and rules structure
├── hooks/                       ← Shared: launcher.sh + launcher.cmd
├── .cursor/rules/               ← Kept as rules-only (no marketplace)
├── .windsurf/rules/             ← Kept as rules-only (no marketplace)
├── .roo/rules/                  ← Kept as rules-only (no marketplace)
├── .clinerules/                 ← Kept as rules-only (no marketplace)
├── .kiro/                       ← Kept steering dir
├── .opencode/                   ← Kept as-is (fork of Kilo)
├── .kilo/                       ← Kept as-is
├── gemini-extension.json        ← Updated with marketplace fields
├── package.json                 ← Updated with Agent Plugins metadata
├── plugin.yaml                  ← Updated with version sync
└── README.md                    ← Updated with marketplace links
```

## Tasks

### 1. Update root `plugin.json` to Agent Plugins v1.0.0

**File:** `plugin.json` (rewrite)
**Steps:**
1.1 Add `$schema` pointing to `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json`
1.2 Add all required fields: `name`, `description`, `author` (name + email + url), `version`, `license`, `keywords`
1.3 Add `homepage`, `repository`
1.4 Move existing hook config to `extensions.com.anthropic.claude-code.hooks`
1.5 Move existing plugin metadata to appropriate extension namespaces
1.6 Update `procoder release` config to include the new file

**Evidence:** File validates as JSON with all required fields. Schema URL matches v1.0.0.

### 2. Create `mcp.json` portable MCP server config

**File:** `mcp.json` (new)
**Steps:**
2.1 Define `$schema` for MCP config
2.2 Create `mcpServers.procorder` with stdio transport pointing to `./dist/procoder` binary
2.3 Include `--mode mcp` as first arg
2.4 Set `cwd` to `${PLUGIN_ROOT}` and env to `${PLUGIN_DATA}/procoder`
2.5 Ensure command path uses `./` relative notation

**Evidence:** File exists with valid JSON schema. `mcpServers` key present with at least one server.

### 3. Restructure existing plugin manifests

**Files:** `.claude-plugin/plugin.json`, `.codex-plugin/plugin.json`, `.devin-plugin/plugin.json`, `gemini-extension.json`, `package.json`
**Steps:**
3.1 `.claude-plugin/plugin.json` — Move hooks from root level to `extensions.com.anthropic.claude-code.hooks` per spec. Add `$schema` and full metadata. Create new top-level directory structure per spec.
3.2 `.codex-plugin/plugin.json` — Update to Agent Plugins format. Move hooks under extension namespace. Add `$schema`, `author`, `version`.
3.3 `.devin-plugin/plugin.json` — Add version, description, hooks config if supported.
3.4 `gemini-extension.json` — Add all marketplace-required fields: version, description, repository, license.
3.5 `package.json` — Add `$schema` for Agent Plugins, author, homepage, repository, keywords. Do NOT rename the `pi` key: pi's own loader reads the `pi` block (`extensions`, `skills`, `prompts`) out of `package.json`, verified against the installed host's package documentation, and renaming it to a namespaced form would leave the extension, the skill path and the gallery keyword unread — the integration that ships today. If the marketplace spec ever demands a different key, the answer is a second key beside this one, not a rename.

**Evidence:** All manifests have consistent metadata (name, version, description, author, license).

### 4. Create client-specific extension directories

**Directories:** `com.anthropic.claude-code/`, `com.kiro/`, `com.cursor/`
**Steps:**
4.1 Create `com.anthropic.claude-code/hooks/claude-hooks.json` with lifecycle hooks (SessionStart, PreToolUse, PostToolUse, PreCompact, Stop)
4.2 Create `com.kiro/hooks/kiro-hooks.json` with Kiro-specific lifecycle events (PreToolUse for gate, PostToolUse for format)
4.3 Create `com.cursor/hooks/cursor-hooks.json` with Cursor-specific hooks if applicable

**Evidence:** Each directory has valid JSON hooks files with proper matcher regex and timeouts.

### 5. Update skills/procoder/SKILL.md with proper frontmatter

**File:** `skills/procoder/SKILL.md` (edit)
**Steps:**
5.1 Ensure YAML frontmatter has: `name`, `description` (1-1024 chars, describes WHAT and WHEN), `license`, `metadata.category`, `metadata.version`
5.2 Ensure name follows spec: 1-64 chars, lowercase alphanumerics + hyphens
5.3 Check body is under 500 lines (progressive disclosure)
5.4 Add `allowed-tools` to frontmatter listing the procoder commands

**Evidence:** YAML frontmatter parses correctly. Description covers both what and when.

### 6. Create VS Code extension scaffold

**Directory:** `vscode/` (new)
**Files:** `vscode/package.json`, `vscode/src/extension.ts` (stub), `vscode/tsconfig.json`, `vscode/README.md`
**Steps:**
6.1 Create `package.json` with all required VS Code fields: `name`, `displayName`, `version`, `publisher`, `description`, `engines.vscode`, `license`, `categories` (at least `["Other"]` or `["Extension Packs"]`), `keywords`
6.2 Add `main` field pointing to compiled output
6.3 Add `contributes` block with:

- `commands`: One command per procoder subcommand (check, format, lint, security, status, spec, plan, todo, test, index, git, release)
- `mcp.servers`: Procorder MCP server declaration
- `configuration`: Settings for procoder binary path, check policy
- `menus`: Context menu items for key operations
  6.4 Add `activationEvents` with `onStartupFinished`
  6.5 Add gallery banner, badges, icon reference
  6.6 Add sponsor link (`github.com/sponsors`)
  6.7 Create minimal `tsconfig.json`
  6.8 Create stub `src/extension.ts` with `activate`/`deactivate` exports
  6.9 Create `README.md` with marketplace description

**Evidence:** `package.json` has all required VS Code fields. `contributes.mcp` declares procoder server.

### 7. Create Cline and Roo Code plugin manifests

**Files:** `cline/plugin.json` (new), `roo/plugin.json` (new)
**Steps:**
7.1 `cline/plugin.json` — Agent Plugins format with name, version, description, author, MCP config pointing to stdio server, skills path
7.2 `roo/plugin.json` — Identical structure to Cline (Roo uses same Agent Plugins spec)
7.3 Both must use `mcp.json`-compatible MCP server definitions

**Evidence:** Both files parse as valid JSON with required fields and MCP config.

### 8. Create GitHub App + Action for GitHub Marketplace

**Files:** `.github/workflows/procoder-gate.yml` (new), `github-app/` directory (new)
**Steps:**
8.1 Create GitHub Action workflow at `.github/workflows/procoder-gate.yml`:

- Triggers: `pull_request`, `push`, `check_run`
- Job runs: `procoder check`, `procoder lint`, `procoder security`
- Posts findings as PR comments
- Sets CI status
  8.2 Create a GitHub App manifest in `github-app/MANIFEST.json`:
- `name`: "procoder"
- `url`: "https://procoder.azrty.com/"
- `hook_attributes`: { `url`, `events`: ["pull_request", "push", "check_run"] }
- `callback_urls`, `redirect_url`
- `public`, `request_oauth_on_install`: true
- `setup_url`
  8.3 Submit the GitHub App on GitHub Marketplace with categories: `code-quality`, `agent-apps`, `code-review`

**Evidence:** Workflow file runs `procoder check` on PRs. GitHub App manifest has all required fields.

### 9. Create formal Cline CLI plugin and Roo plugin

**Files:** `cline/README.md`, `roo/README.md` (new)
**Steps:**
9.1 Create `cline/README.md` with install instructions: `cline install procoder` (hypothetical — verify Cline's actual CLI install pattern)
9.2 Create `roo/README.md` with install instructions: `roo install procoder` (hypothetical — verify Roo's actual install pattern)
9.3 Both READMEs must include: one-line install command, description, use case, and links to docs

**Evidence:** READMEs exist with clear install commands and descriptions.

### 10. Update existing rules directories to be consistent

**Files:** `.cursor/rules/procoder.mdc`, `.windsurf/rules/procoder.md`, `.roo/rules/procoder.md`, `.clinerules/procoder.md`, `.kiro/steering/procoder.md`, `.agents/rules/procoder.md`
**Steps:**
10.1 Verify all rule files point to the same canonical content (AGENTS.md or skills/procoder/SKILL.md)
10.2 Add version references so rules auto-update with skill updates
10.3 Ensure no conflicting instructions across rule files

**Evidence:** All rule files contain identical content (or reference the same source).

### 11. Update `procoder release` config with new files

**File:** `.procoder/config.toml` (edit)
**Steps:**
11.1 Add new version-bearing files to `[release] files`: `plugin.json`, `mcp.json`, `skills/procoder/SKILL.md`, `com.anthropic.claude-code/hooks/claude-hooks.json`, `vscode/package.json`, `cline/plugin.json`, `roo/plugin.json`, `gemini-extension.json`
11.2 Remove or keep old files as appropriate

**Evidence:** Config.toml includes all version-bearing files. `procoder release` validates them.

### 12. Create marketplace submission documentation

**File:** `docs/marketplace-submissions.md` (new) or `.procoder/release/` (subdirectory)
**Steps:**
12.1 Document each marketplace with: URL, submission process, status (pending/submitted/accepted/rejected), evidence link
12.2 Track Claude Code community marketplace submission
12.3 Track Kiro Powers marketplace submission
12.4 Track VS Code Marketplace submission
12.5 Track GitHub Marketplace submission
12.6 Document any blockers or requirements for each

**Evidence:** Submission tracking document exists with status for each marketplace.

### 13. Validate and test all changes

**Steps:**
13.1 Run `procoder check` — gate must pass (no conflicts from new files)
13.2 Run `procoder test` — suite must pass
13.3 Run `procoder format` on all new/modified files
13.4 Run `procoder lint` — no blocking findings
13.5 Verify all JSON files parse correctly (no syntax errors)
13.6 Verify YAML frontmatter in SKILL.md parses correctly

**Evidence:** `procoder check`, `procoder test`, `procoder lint` all pass.

## Dependencies

- Task 1 must complete before Task 3 (restructured manifest is the source of truth)
- Task 1 must complete before Task 4 (extension namespaces depend on schema URL)
- Task 5 and Task 3 are independent (frontmatter vs manifest metadata)
- Task 6 is independent (VS Code is a separate ecosystem)
- Task 7 is independent (Cline/Roo use same Portable spec as task 1)
- Task 8 is independent (GitHub is a separate ecosystem with its own model)
- Task 13 last (validation after all changes)

## Risk Assessment

| Risk                                                        | Impact                  | Mitigation                                                       |
| ----------------------------------------------------------- | ----------------------- | ---------------------------------------------------------------- |
| Claude Code marketplace review is slow or rejected          | Blocks Task 3c evidence | Self-distribute via GitHub; submission is one way                |
| VS Code extension requirements are stricter than documented | Blocks Task 6           | Start with minimal extension that only declares MCP server       |
| Cline/Roo install patterns differ from assumptions          | Blocks Task 9           | Document what we know, update after testing                      |
| Restructuring breaks existing users' integrations           | Blocks Task 3, 10       | Keep old directory structure alongside new one; deprecate slowly |
| `procoder release` config update breaks release process     | Blocks Task 11          | Test on a branch first; keep original files in sync              |
