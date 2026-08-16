# Changelog

## 0.1.0

Initial release.

- The four-rung doctrine — **SAFE**, **TRUE**, **OBVIOUS**, **ALONE** — as the
  single source in `skills/procoder/SKILL.md`.
- Three intensity levels (`pragmatic`, `strict`, `paranoid`), persisted across
  sessions and switchable with `/procoder <level>`.
- A deterministic PostToolUse check engine covering six languages
  (TypeScript/JavaScript, Python, Go, Rust, JVM, .NET), with external-linter
  deference where a project already has one configured.
- A ratchet baseline (`procoder baseline` / `procoder verify`): accepted debt
  may shrink, never grow.
- Ten slash commands: `/procoder`, `/procoder-review`, `/procoder-audit`,
  `/procoder-rot`, `/procoder-threat`, `/procoder-deps`, `/procoder-debt`,
  `/procoder-gain`, `/procoder-guard`, `/procoder-help`.
- An MCP server (`procoder-mcp/server.js`) exposing the doctrine and check
  engine over JSON-RPC for non-Claude-Code hosts.
- Generated platform rule files for Cursor, Windsurf, Cline, Kiro, Qoder,
  generic `.agents`, `AGENTS.md`, and openclaw — eight in all — plus ten
  command ports each for opencode and openclaw, all rendered from the single
  doctrine source with a CI drift gate (`npm run sync:check`).
- Worked before/after examples for every rung (`examples/`) and per-host
  install instructions (`docs/install.md`).
- Known limitations, verified against the current source and written up
  plainly rather than left for users to discover: see
  [`docs/known-limitations.md`](docs/known-limitations.md).
