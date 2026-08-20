---
description: "The maintainability report: dead-code candidates, complexity, function length — judgment calls you decide on."
---

The user invoked /procoder:maintain.

The command below is the `procoder` binary on PATH.

1. Run `procoder index build` if the index is missing or stale (stats
   tells you), then `procoder maintain`.
2. Judge every line — nothing here blocks, everything is a candidate:
   - **unused**: dead-code candidates from the precise index. Exported API
     is marked — a library's public surface is legitimately unreferenced
     from inside; internal symbols with no references usually deserve
     deletion. Confirm with `procoder index refs <symbol>` before
     removing anything.
   - **complexity / funlen**: long or deeply-branched functions. Refactor
     the ones that are genuinely hard to follow; say why for the ones you
     leave (a table-driven function can be long and clear).
3. Deletion is the best refactor: prefer removing dead code over commenting
   it out or keeping it "for later".
4. The thresholds are the repo's to set: `[maintain] gocyclo`,
   `funlen_lines`, `funlen_statements` in .procoder/config.toml
   (defaults 15/80/50).
5. Report to the user what you removed, what you refactored, and what you
   judged fine as-is.
