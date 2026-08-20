# Every agent

**Reference.** Procoder works with any AI coding agent, not just Claude
Code. One
canonical `AGENTS.md` carries the always-on contract; every host gets the
thinnest adapter that serves it — a rule-file copy, a manifest pointing
at the existing files, or a hook. `procoder agents` prints anything
missing or drifted, and drift blocks the gate like any other mirror.

The adapter rule: adapters stay thin. Logic lives in the binary; content
lives in `AGENTS.md` and `commands/`; an adapter only points or copies.

## Instruction tier — a rules file, nothing else

| Host                                     | File                              | Install                 |
| ---------------------------------------- | --------------------------------- | ----------------------- |
| Zed, Amp, Jules, Swival, VS Code agents… | `AGENTS.md`                       | nothing — read natively |
| Cursor                                   | `.cursor/rules/procoder.mdc`      | copy into your repo     |
| Windsurf                                 | `.windsurf/rules/procoder.md`     | copy into your repo     |
| Cline                                    | `.clinerules/procoder.md`         | copy into your repo     |
| Kilo Code                                | `.kilo/rules/procoder.md`         | copy into your repo     |
| Kilo Code (legacy path)                  | `.kilocode/rules/procoder.md`     | copy into your repo     |
| Roo Code                                 | `.roo/rules/procoder.md`          | copy into your repo     |
| Kiro                                     | `.kiro/steering/procoder.md`      | copy into your repo     |
| Antigravity                              | `.agents/rules/procoder.md`       | copy into your repo     |
| Qoder                                    | `.qoder/rules/procoder.md`        | copy into your repo     |
| Copilot in editors                       | `.github/copilot-instructions.md` | copy into your repo     |
| OpenAI Codex (repo docs)                 | `.codex/AGENTS.md`                | copy into your repo     |
| Any skill-aware host                     | `skills/procoder/SKILL.md`        | copy into your repo     |

Every copy is byte-pinned to `AGENTS.md` (host frontmatter aside) by the
gate; edit the master, then `procoder agents` prints the refreshed copies.

## Plugin tier — manifests and hooks

| Host                     | Entry                                               | Notes                                                                                                                                                            |
| ------------------------ | --------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Claude Code              | `.claude-plugin/plugin.json`                        | the reference host — fully tested                                                                                                                                |
| Codex CLI                | `.codex-plugin/plugin.json`                         | shares Claude's hooks file; the binary answers in Codex's JSON shape by host detection                                                                           |
| GitHub Copilot CLI       | `.github/plugin/plugin.json`                        | own hook schema (`hooks/copilot-hooks.json`, bash+powershell); session-start injection only                                                                      |
| Gemini CLI / Antigravity | `gemini-extension.json`                             | `contextFileName: AGENTS.md`; deliberately no root `hooks/hooks.json` (Gemini would auto-load it with incompatible events — its absence is enforced by the gate) |
| OpenCode                 | `opencode.json` → `.opencode/plugins/procoder.mjs`  | JS shim injects `AGENTS.md` per turn; `.opencode/command/*.md` are generated twins of `commands/*.md` (parity pinned by test)                                    |
| Grok Build               | root `plugin.json`                                  | skills only, no hooks                                                                                                                                            |
| Devin CLI                | `.devin-plugin/plugin.json`                         | metadata + skills                                                                                                                                                |
| Qoder (plugin tier)      | `.qoder-plugin/plugin.json`                         | skills + rules pointer                                                                                                                                           |
| pi                       | `package.json` `pi` block → `pi-extension/index.js` | injects `AGENTS.md` at agent start                                                                                                                               |
| Hermes Agent             | `plugin.yaml` + `__init__.py`                       | `pre_llm_call` hook injects `AGENTS.md`                                                                                                                          |

Host detection lives in the binary (`internal/host`): `COPILOT_PLUGIN_DATA`
(or a VS Code plugin-root path) → Copilot, `PLUGIN_DATA` → Codex,
`QODER_SESSION_ID` → Qoder, else Claude. `procoder principles --hook`
answers in each host's session-start shape.

## Test coverage per host

Claude Code is the reference host — every command, hook, and skill is
exercised there daily. The other adapters follow each host's published
plugin shape (largely proven in the wild by the ponytail plugin's
portability layer, which this design follows) and are not continuously
tested against live installs. If one misbehaves on your host, that is a
bug — report it with the host name and the adapter file.

Manifest versions are pinned to `.claude-plugin/plugin.json` by the gate,
so a release can never leave one host stale.

## The skill tier — one skill, every skill-aware host

`skills/procoder/SKILL.md` is the AGENTS.md body under the Agent Skills
envelope ([agentskills.io](https://agentskills.io/specification)). Hosts
that scan a skills directory load it on demand instead of always-on:
Kilo reads `.kilo/skills/`, `.claude/skills/`, and `.agents/skills/`, and
the Kilo Marketplace indexes this path as the canonical source. It is
pinned to `AGENTS.md` by the same drift check as every rule copy — the
frontmatter is the only part that is its own.

## Where the gate actually blocks

The instruction tier asks; only a hook refuses. Today the commit gate
blocks in Claude Code and Codex (`hooks/claude-hooks.json`), in Copilot
CLI (`hooks/copilot-hooks.json`), and — through the shared JS shim — in
OpenCode and Kilo, where `tool.execute.before` throws on a deny verdict.
Every one of them reaches the same `procoder hook pre-tool-use` entry
point, so there is one gate implementation and one verdict, whatever the
host. A host without the binary is told the gate did NOT run rather than
having its commits blocked.

## The everywhere-binary

All tiers assume the `procoder` binary is reachable. Claude Code gets it
via the plugin automatically; for every other host, put the right
`dist/<platform>/procoder` on PATH (or `go install`). The instruction
tier degrades gracefully: without the binary the rules still shape
behaviour, and the agent is told what to install.
