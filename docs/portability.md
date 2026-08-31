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

| Host                     | Entry                                                | Notes                                                                                                                                                                                                                                                                                                                                          |
| ------------------------ | ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Claude Code              | `.claude-plugin/plugin.json`                         | the reference host — fully tested                                                                                                                                                                                                                                                                                                              |
| Codex CLI                | `.codex-plugin/plugin.json`                          | shares Claude's hooks file; the binary answers in Codex's JSON shape by host detection                                                                                                                                                                                                                                                         |
| GitHub Copilot CLI       | `.github/plugin/plugin.json`                         | own hook schema (`hooks/copilot-hooks.json`, bash+powershell); session-start injection only                                                                                                                                                                                                                                                    |
| Gemini CLI / Antigravity | `gemini-extension.json`                              | `contextFileName: AGENTS.md`; deliberately no root `hooks/hooks.json` (Gemini would auto-load it with incompatible events — its absence is enforced by the gate)                                                                                                                                                                               |
| OpenCode                 | `opencode.json` → `.opencode/plugins/procoder.mjs`   | JS shim injects `AGENTS.md` per turn; `.opencode/command/*.md` are generated twins of `commands/*.md` (parity pinned by test). The shim's extensions are load-bearing: both hosts discover `{plugin,plugins}/*.{ts,js}`, and this file loads only because `opencode.json` names it explicitly — the file's header records what a rename breaks |
| Grok Build               | root `plugin.json`                                   | skills only, no hooks                                                                                                                                                                                                                                                                                                                          |
| Devin CLI                | `.devin-plugin/plugin.json`                          | metadata + skills                                                                                                                                                                                                                                                                                                                              |
| Qoder (plugin tier)      | `.qoder-plugin/plugin.json`                          | skills + rules pointer                                                                                                                                                                                                                                                                                                                         |
| pi                       | `package.json` `pi` block → `pi-extension/index.mjs` | every hook surface: `before_agent_start` injects the contract only when pi has not loaded `AGENTS.md` itself, `tool_call` gates a `git commit`, `tool_result` carries the write hook's findings, `agent_settled` writes the handoff; commands register at load as `/procoder:*`, and `skills/` is the skill path                               |
| Hermes Agent             | `plugin.yaml` + `__init__.py`                        | `pre_llm_call` hook injects `AGENTS.md`                                                                                                                                                                                                                                                                                                        |

Host detection lives in the binary (`internal/host`): `COPILOT_PLUGIN_DATA`
(or a VS Code plugin-root path) → Copilot, `PLUGIN_DATA` → Codex,
`QODER_SESSION_ID` → Qoder, `PI_CODING_AGENT` → pi, else Claude. `procoder
principles --hook` answers in each host's session-start shape.

### Where pi gets more than the reference host

"Supported on host X" is not one claim across this table, and pretending it is
would hide three places where pi has more than Claude Code:

- `tool_result` lets the write hook's findings be patched into the result of the
  write that caused them. Claude Code's PostToolUse hands the model an
  `additionalContext` block beside the result, where somebody has to connect it
  back to a file.
- `agent_settled` fires whenever pi will not continue on its own, so the handoff
  note and the unasked-decision check run at the end of a turn. Claude Code
  learns a turn ended from `Stop`, and learns a compaction is coming only from
  `PreCompact`.
- the gate is also a callable tool. `check`, `test`, `lint`, `security`, `debt`,
  `status`, `review`, `doctor` and `index` run without a shell, so the report a
  session reads cannot come from a different version than the one the package
  shipped. Mutating verbs are refused there: closing work, seeding the backlog
  and releasing stay slash commands a human types.

Everything else — the gate verdict, the format result, the secret findings, the
handoff format, the unasked-decision rule and its dedupe — is the same binary
entry points Claude Code calls. The adapter decides none of it.

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

## The trailer your host adds

Several hosts append an attribution trailer to every commit message they
write — `Co-Authored-By: Claude <noreply@anthropic.com>`, a "Generated
with …" line, a robot emoji, `Co-authored-by: Codex <noreply@openai.com>`.
The gate blocks the ones it recognises, and blocks them without a knob:
the work is the author's, and a repository that wants the rule off
already has `[git] commit_gate`.

What it recognises is a named list of machine authors — Claude, Codex,
Copilot, Cursor, Devin, Gemini, aider, and the robot emoji — matched on
the trailer that names one, plus the "Generated with …" and vendor
`noreply@` forms. Not every `Co-Authored-By:`: the header is a decade
older than AI coders and is right for pair programming, a patch carried
on someone's behalf, or a squash that credits everyone who touched the
branch, so those keep passing. The finding names which identity it
matched, so a wrong one can be argued with rather than worked around. A
host outside the list is a silent miss and worth an issue.

That makes the trailer a recurring wall rather than a one-time fix.
Amending the message clears the finding; the host writes the trailer
again on the next commit, because the setting that produced it never
changed. The fix belongs in the host, not in the commit.

### Settings that turn it off

| Host                           | Setting                                                                               |
| ------------------------------ | ------------------------------------------------------------------------------------- |
| Claude Code                    | `attribution` in `settings.json` — `commit` and `pr` to `""`, `sessionUrl` to `false` |
| VS Code, and Copilot inside it | `git.addAICoAuthor` to `"off"`                                                        |
| Codex                          | account or workspace policy, not a local setting — see below                          |

Claude Code reads `attribution` from any settings file it loads
(`~/.claude/settings.json` for every project, `.claude/settings.json` to
commit the choice with the repository, `.claude/settings.local.json` to
keep it to yourself):

```json
{
  "attribution": { "commit": "", "pr": "", "sessionUrl": false }
}
```

`commit` covers the trailer, `pr` the pull-request body, and `sessionUrl`
the session link appended as its own trailer. The setting supersedes the
deprecated `includeCoAuthoredBy`.

`git.addAICoAuthor` is VS Code's own git setting and takes `off`,
`chatAndAgent`, or `all`; `off` is the one that stops the trailer. Editors
forked from VS Code may carry the same key under the same name — worth
trying before assuming they do not.

Codex resolves attribution as a policy from the account rather than from
a config file, and the instruction it injects explicitly outranks any
request to drop the trailer. Turn it off where the account or workspace
is administered; there is no `config.toml` key for it.

### Hosts without a verified setting

Every other host in the tables above is **unverified**. That means no
setting has been confirmed against the host's own documentation or
source — not that none exists. A key printed here on a guess would cost
more than the gap it filled. To settle it for your host:

1. Let the host write one commit, then read `git log -1 --format=%B`. No
   trailer, nothing to turn off.
2. Search the host's own settings for `attribution`, `co-author`, or
   `coAuthored` — that is where such a key is named when it exists.
3. Failing both, the repository's own defence still holds: the gate
   refuses the commit, and `procoder scrub` catches the same lines in a
   drafted PR body before it is sent.

A host you confirm either way is worth an issue, with the host name and
the setting.

## The everywhere-binary

All tiers assume the `procoder` binary is reachable. Claude Code gets it
via the plugin automatically; for every other host, put the right
`procoder` from the release on PATH (or `go install`). The instruction
tier degrades gracefully: without the binary the rules still shape
behaviour, and the agent is told what to install.

## Windows

Windows reaches the binary through the same `hooks/launcher.sh` every
other platform uses. Claude Code — and Codex CLI, which shares the hooks
file — runs hook commands through Git Bash, where `uname -s` answers
`MINGW64_NT-10.0-26200`; MSYS2 and Cygwin shells answer in the same
family. The launcher matches `MINGW*`, `MSYS*` and `CYGWIN*` and
resolves the cached `dist/windows-amd64/procoder.exe`, fetching it once
if it is not there yet, so every hook and every
slash command works with no manifest change and no per-host wiring.

Copilot CLI is the exception on Windows only: `hooks/copilot-hooks.json`
calls `hooks/launcher.sh` like everything else, and falls back to a
PowerShell branch that names the `.exe` itself where no POSIX shell is
available.

Only `windows-amd64` ships. On ARM64 Windows, Git Bash reports the
emulated `x86_64` and that binary runs. A POSIX shell reporting
`aarch64`, or `launcher.cmd` seeing `PROCESSOR_ARCHITECTURE=ARM64`, is
told there is no binary for `windows/arm64` rather than being handed the
wrong one.
