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

To see the active level in your status line:

```bash
procoder statusline install
```

That edits Claude Code's settings for you — `$CLAUDE_CONFIG_DIR/settings.json`,
or `~/.claude/settings.json` — and works out the script path itself, which
matters because Claude Code plugin installs do not live at a fixed, predictable
location. It picks `procoder-statusline.sh` (via bash) or
`procoder-statusline.ps1` (via powershell) for your platform.

It is careful with that file, because it is yours and procoder only owns one key
in it: every other key keeps its value and its position, the previous version is
copied to `settings.json.backup-<timestamp>` before anything is written, and the
new file is written to a temp file and renamed, so an interrupted run cannot
leave a truncated `settings.json` behind. If the file is not valid JSON, the
command says so and changes nothing rather than overwriting what it could not
read. If a `statusLine` is already configured and it is not procoder's, it
reports both and stops; `--force` replaces it.

```bash
procoder statusline status     # what is configured today
procoder statusline uninstall  # remove procoder's entry, leave everything else
```

### Doing it by hand

The command is the recommended path, but the block is small enough to write
yourself — and you will need to, if your install path contains characters a
shell would interpret (`$`, a backtick, a double quote, a backslash), because
the installer refuses to build a command around one and prints this instead:

```json
{
  "statusLine": {
    "type": "command",
    "command": "bash /path/to/procoder/hooks/procoder-statusline.sh"
  }
}
```

`/path/to/procoder` is a placeholder — substitute the actual install path,
quoted or escaped to suit your shell. On Windows, the command is
`powershell -NoProfile -File C:\path\to\procoder\hooks\procoder-statusline.ps1`.

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
procoder check <paths...>     # exit 1 if any non-baselined finding blocks at the active level
procoder baseline <paths...>  # record current findings as accepted
procoder verify <paths...>    # exit 1 if any finding isn't in the baseline — the CI ratchet
```

At `pragmatic`, `check` reports OBVIOUS and ALONE findings but does not exit 1
on them; every other level gates all four rungs, and so does CI, which has no
level file and therefore resolves to the default `strict`.

`verify` takes one extra flag, `--unused-exclusions`: it also fails if an
`[exclude] rules` entry suppressed nothing in this run — a stale suppression
left behind after the finding it silenced was fixed. Without the flag the stale
entries are still reported, they just do not fail the build.

## Troubleshooting

**The PostToolUse hook doesn't seem to run — no findings ever appear.**

- Check `PROCODER_NO_HOOK`. Every hook entry point (`procoder-activate.js`,
  `procoder-check.js`, `procoder-subagent.js`, `procoder-mode-tracker.js`)
  exits immediately, before doing anything, when `PROCODER_NO_HOOK=1` is set
  in the environment. This is meant for CI and test runs that shell out to
  Claude Code without wanting procoder in the loop — if it's set in your
  interactive shell by habit or by a dotfile, the hook is a silent no-op.
- Check the level file: `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/.procoder-active`.
  If it holds `off` (written by `/procoder off` or the deactivation phrase),
  the hook still runs but treats the session as inactive. Delete the file or
  set a real level with `/procoder <level>` to re-activate.
- Check that the plugin's hooks are actually registered — `claude plugin
  list` should show `procoder` installed, and `hooks/claude-hooks.json` in
  the installed plugin should be the source Claude Code loaded. A plugin
  installed from a stale local path (`claude plugin marketplace add
  ./procoder` pointed at an old checkout) registers hooks from that old
  checkout, not your current source tree.

**The statusline shows nothing.**

`hooks/procoder-statusline.sh` (or the `.ps1` equivalent) prints nothing on
purpose whenever it can't read
`${CLAUDE_CONFIG_DIR:-$HOME/.claude}/.procoder-active`, or the file's content
isn't one of `pragmatic`, `strict`, or `paranoid`. Confirm: `procoder statusline
status` reports procoder's command as configured — if it reports nothing, or
somebody else's statusline, run `procoder statusline install`; the level file
exists and holds a real level, not `off` or empty; and the script is readable at
the path the command names. A settings file written by hand against an older
install can point at a path that no longer exists — re-running the install
command rewrites it from where procoder lives now.

**A wall of findings on first use.**

Expected on any repo with pre-existing debt — procoder was not run at
whatever earlier point that debt was written. This is what `procoder
baseline <paths...>` is for: it records every current finding as accepted, so
only new and changed code is gated going forward. See [Known
limitations](known-limitations.md) for what the baseline's fingerprinting
does and doesn't protect against.

**A finding on a line that only *describes* a violation.**

Documentation quoting an injection sink, a test fixture holding a real-shaped
credential, a rule id named in config — all read to a regex exactly like the
thing they talk about. Mark the line instead of excluding the file:

`<comment syntax> procoder: literal <rule-id>[, <rule-id>…] <reason>` <!-- procoder: literal alone/blanket-suppression the marker syntax written out, not a suppression -->

Trailing on a line it covers that line; standing on its own line it covers that
line and the next. It must name its rules and give a reason, or it silences
nothing and is itself reported. See the README's Configuration section for the
full rules, and [Known limitations](known-limitations.md) for what it cannot
reach — notably external linter rule ids such as `true/eslint:no-eval`.

**A version was released, but the session said nothing about updating.**

Working as designed, in the common case. The SessionStart hook never waits on
the network: it reads a cached answer, and only when that answer is more than a
day old does it spawn a detached background process to fetch a fresh one, which
lands for the *next* session. So the first session after a release is silent and
the next one carries the notice. The alternative — a blocking HTTPS request
inside a hook with a 5 second timeout, on every session — puts a slow or captive
network between you and the prompt in exchange for a cosmetic message.

If it never appears at all:

- The notice is only for plugin installs. It is skipped entirely unless
  `CLAUDE_PLUGIN_ROOT` is set, which Claude Code sets when it runs the hook — a
  source checkout or a hand-run hook neither notifies nor makes any request.
- Check `PROCODER_NO_UPDATE_CHECK` and `PROCODER_NO_HOOK`. Either set to `1`
  suppresses it, background request included.
- Inspect the cache:
  `${CLAUDE_CONFIG_DIR:-$HOME/.claude}/.procoder-update-check.json`. It holds
  `checkedAt` and the last `latest` seen. Deleting it makes the next session
  refresh. If `checkedAt` keeps moving but `latest` never does, the fetch is
  failing — a proxy, a firewall, or no route to
  `raw.githubusercontent.com`. Every such failure is deliberately silent; set
  `PROCODER_UPDATE_URL` to an internal mirror if that host is unreachable
  by policy.
- procoder never notifies you downwards. If your installed version is *newer*
  than the published one, or either version string is not `MAJOR.MINOR.PATCH`,
  the notice is suppressed rather than guessed at.

**`procoder verify` exits 2 instead of 0 or 1.**

Exit 2 means "cannot verify," not "the ratchet grew." It happens when the
baseline file (`.procoder-baseline.json` by default) was written by an older,
incompatible version of procoder — the fingerprint format changed between
baseline format versions and old entries cannot be migrated, so nothing in
the stale file is honored. The fix is exactly what the stderr message says:
run `procoder baseline <paths...>` to write a current-format baseline, then
re-run `verify`.

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
