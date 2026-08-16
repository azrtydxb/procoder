# Changelog

## 0.1.0

Initial release.

- The four-rung doctrine — **SAFE**, **TRUE**, **OBVIOUS**, **ALONE** — as the
  single source in `skills/procoder/SKILL.md`.
- Three intensity levels (`pragmatic`, `strict`, `paranoid`), persisted across
  sessions and switchable with `/procoder:level <level>`.
- A deterministic PostToolUse check engine covering six languages
  (TypeScript/JavaScript, Python, Go, Rust, JVM, .NET), with external-linter
  deference where a project already has one configured.
- A line marker for text that describes a violation rather than committing
  one: `procoder: literal <rule-id>… <reason>` <!-- procoder: literal alone/blanket-suppression the marker syntax written out, not a suppression -->
  It must name its rules and
  state a reason; it reaches one line, or two in its standalone form. It is the
  only suppression form narrower than `.procoder.toml`'s `[exclude] rules`.
- Dependency-manifest checks: floating version ranges and missing lockfiles.
- A ratchet baseline (`procoder baseline` / `procoder verify`): accepted debt
  may shrink, never grow. `verify --unused-exclusions` also fails on an
  `[exclude] rules` entry that has stopped suppressing anything.
- Bounded reporting: 20 findings per line, with the overflow reported as a
  finding rather than dropped; a 4MB per-file skip and a 2s budget, both
  announced on stderr when they bite.
- A `.procoder.toml` subset parser that warns on stderr for syntax it cannot
  handle, so a mis-parsed exclusion cannot silently narrow the gate.
- Ten slash commands: `/procoder:level`, `/procoder:review`, `/procoder:audit`,
  `/procoder:rot`, `/procoder:threat`, `/procoder:deps`, `/procoder:debt`,
  `/procoder:gain`, `/procoder:guard`, `/procoder:help`.
- An MCP server (`procoder-mcp/server.js`) exposing the doctrine and check
  engine over JSON-RPC for non-Claude-Code hosts.
- Generated platform rule files for Cursor, Windsurf, Cline, Kiro, Qoder,
  generic `.agents`, `AGENTS.md`, and openclaw — eight in all — plus ten
  command ports each for opencode and openclaw, all rendered from the single
  doctrine source with a CI drift gate (`npm run sync:check`).
- Worked before/after examples for every rung (`examples/`) and per-host
  install instructions (`docs/install.md`).
- A self-scan (`tests/dogfood.test.js`) over the whole tracked tree as reported
  by `git ls-files`, with two documented hold-outs: historical planning
  documents, and the `examples/*/before.*` files that violate a rung on
  purpose.
- Known limitations, verified against the current source and written up
  plainly rather than left for users to discover: see
  [`docs/known-limitations.md`](docs/known-limitations.md).
