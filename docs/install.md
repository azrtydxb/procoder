# Installing procoder

procoder's doctrine lives in `skills/procoder/SKILL.md` and is generated into a
rule file for every host below by `scripts/sync-rules.js`. Ten slash commands
are ported the same way to `.opencode/command/` and `.openclaw/commands/`. CI
fails if any generated file drifts from its source, so every path below reads
the same doctrine.

## Claude Code

```bash
claude plugin marketplace add azrtydxb/procoder
claude plugin install procoder
```

(Replace the marketplace source with a local path if you're installing from a
clone: `claude plugin marketplace add ./procoder`.)

To see the active level in your status line, add this to Claude Code's
settings, pointing at wherever the plugin is installed on your machine:

```json
{
  "statusLine": {
    "type": "command",
    "command": "bash /path/to/procoder/hooks/procoder-statusline.sh"
  }
}
```

`/path/to/procoder` is a placeholder — substitute the actual install path on
your machine (Claude Code plugin installs do not live at a fixed, predictable
location). A PowerShell equivalent, `procoder-statusline.ps1`, is included for
Windows — use `powershell -File /path/to/procoder/hooks/procoder-statusline.ps1`.

## Cursor

Copy or symlink the generated rule file into your project:

```bash
mkdir -p .cursor/rules
cp /path/to/procoder/.cursor/rules/procoder.mdc .cursor/rules/procoder.mdc
```

Cursor reads every `.mdc` file under `.cursor/rules/`; `alwaysApply: true` in
its frontmatter means procoder applies to every request, not just ones that
reference it.

## Windsurf

```bash
mkdir -p .windsurf/rules
cp /path/to/procoder/.windsurf/rules/procoder.md .windsurf/rules/procoder.md
```

## Cline

```bash
mkdir -p .clinerules
cp /path/to/procoder/.clinerules/procoder.md .clinerules/procoder.md
```

## Kiro

```bash
mkdir -p .kiro/steering
cp /path/to/procoder/.kiro/steering/procoder.md .kiro/steering/procoder.md
```

## Qoder

```bash
mkdir -p .qoder/rules
cp /path/to/procoder/.qoder/rules/procoder.md .qoder/rules/procoder.md
```

## opencode

opencode reads `AGENTS.md` for the doctrine, and `.opencode/command/*.md` for
the ten slash commands:

```bash
cp /path/to/procoder/AGENTS.md AGENTS.md
mkdir -p .opencode/command
cp /path/to/procoder/.opencode/command/*.md .opencode/command/
```

`opencode.json` (a bare `$schema` pointer) ships at the repo root and needs no
further setup.

## openclaw

```bash
mkdir -p .openclaw/skills .openclaw/commands
cp -r /path/to/procoder/.openclaw/skills/procoder .openclaw/skills/procoder
cp /path/to/procoder/.openclaw/commands/*.md .openclaw/commands/
```

## Codex / GitHub Copilot

Both read `AGENTS.md` directly — copy or symlink it into the project root:

```bash
cp /path/to/procoder/AGENTS.md AGENTS.md
```

The hooks (activation, level tracking) speak each host's own wire protocol via
the `PROCODER_HOST` environment variable, since Codex and Copilot do not use
Claude Code's hook JSON:

```bash
export PROCODER_HOST=codex     # or: copilot
```

Codex is also auto-detected via the `CODEX_HOME` environment variable it
already sets, so `PROCODER_HOST=codex` is rarely needed in practice.

## pi

```bash
npm install -g pi
```

Then register procoder as a pi extension — `package.json` declares the entry
point already:

```json
{ "pi": { "skills": ["./skills"], "extensions": ["./pi-extension/index.js"] } }
```

pi resolves `./pi-extension/index.js` relative to wherever procoder is
installed on disk.

## MCP

`procoder-mcp/server.js` is a dependency-free JSON-RPC 2.0 (stdio,
newline-delimited) server exposing the same check engine the hooks use. Point
any MCP-speaking host at it:

```json
{
  "mcpServers": {
    "procoder": {
      "command": "node",
      "args": ["/path/to/procoder/procoder-mcp/server.js"]
    }
  }
}
```

It answers `initialize`, `tools/list`, and three `tools/call` targets:
`procoder_doctrine` (the rungs, filtered to a level), `procoder_check` (run the
engine against a file), and `procoder_baseline` (read the ratchet baseline).

## CLI / CI only

For hosts with no agent to read a rule file at all — a pre-commit hook, a CI
job, a plain terminal:

```bash
npm install -g procoder
```

Then either run `/procoder-guard` inside a session that has the plugin
installed, which writes the pre-commit hook and CI export for you, or wire the
CLI directly:

```bash
procoder check <paths...>     # exit 1 if any non-baselined finding exists
procoder baseline <paths...>  # record current findings as accepted
procoder verify <paths...>    # exit 1 if any finding isn't in the baseline — the CI ratchet
```

## Host reference

| Host | File it reads | Supports levels |
|---|---|---|
| Claude Code | plugin hooks (`hooks/claude-hooks.json`) | Yes — `/procoder <level>`, persisted |
| Cursor | `.cursor/rules/procoder.mdc` | No — doctrine is rendered at `strict` |
| Windsurf | `.windsurf/rules/procoder.md` | No — doctrine is rendered at `strict` |
| Cline | `.clinerules/procoder.md` | No — doctrine is rendered at `strict` |
| Kiro | `.kiro/steering/procoder.md` | No — doctrine is rendered at `strict` |
| Qoder | `.qoder/rules/procoder.md` | No — doctrine is rendered at `strict` |
| Generic `.agents` convention | `.agents/rules/procoder.md` | No — doctrine is rendered at `strict` |
| opencode | `AGENTS.md` + `.opencode/command/*.md` | No — doctrine is rendered at `strict` |
| openclaw | `.openclaw/skills/procoder/SKILL.md` + `.openclaw/commands/*.md` | No — doctrine is rendered at `strict` |
| Codex / Copilot | `AGENTS.md`, hooks via `PROCODER_HOST` | Yes, where the host's hook protocol supports it |
| pi | `pi-extension/index.js` → `skills/` | Depends on pi's own skill invocation |
| MCP | `procoder-mcp/server.js` (`procoder_doctrine` tool) | Yes — pass `level` as a tool argument |
| CLI / CI | `bin/procoder.js` directly | N/A — deterministic checks only, no doctrine text |
