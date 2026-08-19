---
description: "The code index: build it, then find, search, refs, outline, and impact instead of grepping blind."
---

The user invoked /procoder:index with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

The index is your fast map of the codebase — reach for it before grepping:

- `procoder index build` — build both tiers (universal-ctags broad,
  SCIP precise where the language has an indexer). Run this first, and
  rerun when stats says the index is stale.
- `procoder index find <symbol>` — where a symbol is defined
  (file:line, kind, signature).
- `procoder index search <text>` — fuzzy symbol search when you don't
  know the exact name.
- `procoder index refs <symbol>` — every reference; the output says
  whether it answered precise (SCIP) or textual (git grep).
- `procoder index outline <file>` — a file's symbols in order; read
  this before reading the whole file.
- `procoder index impact` — which symbols the working-tree change
  defines and which files reference them; verify those files before
  calling the work finished.
- `procoder index callers <symbol>` — who calls it and what it calls
  (precise tier).
- `procoder index unused` — dead-code candidates: defined, never
  referenced; exported API is marked, you judge it.
- `procoder index entrypoints` — main functions and the exported
  surface, the security starting set.
- `procoder index graph` — the full caller→callee edge list as JSON,
  for tools that walk reachability.
- `procoder index stats` — what's indexed and how fresh.

If a command answers "no index" or names missing tools, run
`procoder init` and then `procoder index build`. Staleness notes in
the output are real — refresh before trusting line numbers.

With arguments, run the matching subcommand directly and act on its output.
Without arguments, run `stats` and report the index's state.
