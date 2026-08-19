---
description: "The code index: build it, then find, search, refs, outline, and impact instead of grepping blind."
---

The user invoked /procoder:index with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

The index is your fast map of the codebase — reach for it before grepping:

- `launcher.sh index build` — build both tiers (universal-ctags broad,
  SCIP precise where the language has an indexer). Run this first, and
  rerun when stats says the index is stale.
- `launcher.sh index find <symbol>` — where a symbol is defined
  (file:line, kind, signature).
- `launcher.sh index search <text>` — fuzzy symbol search when you don't
  know the exact name.
- `launcher.sh index refs <symbol>` — every reference; the output says
  whether it answered precise (SCIP) or textual (git grep).
- `launcher.sh index impls <symbol>` — what implements an interface or
  its methods (precise tier only; it says so when that tier is absent).
- `launcher.sh index outline <file>` — a file's symbols in order; read
  this before reading the whole file.
- `launcher.sh index impact` — which symbols the working-tree change
  defines and which files reference them; verify those files before
  calling the work finished.
- `launcher.sh index callers <symbol>` — who calls it and what it calls
  (precise tier).
- `launcher.sh index unused` — dead-code candidates: defined, never
  referenced; exported API is marked, you judge it.
- `launcher.sh index entrypoints` — main functions and the exported
  surface, the security starting set.
- `launcher.sh index graph` — the full caller→callee edge list as JSON,
  for tools that walk reachability.
- `launcher.sh index stats` — what's indexed and how fresh.
- `launcher.sh index rename <symbol> <newname> [--at path:line]` — the
  cross-file rename as a reviewable unified diff (Go, computed by gopls;
  other languages get the reference worksheet instead). Nothing is
  written: review the diff, apply it yourself, then verify with
  `index refs <newname>` and `lint --types` where it applies.

If a command answers "no index" or names missing tools, run
`launcher.sh init` and then `launcher.sh index build`. Staleness notes in
the output are real — refresh before trusting line numbers.

The index is the fast, repo-wide map; a live language server is the
microscope. When a `refs` answer is textual or same-named symbols
collapse together, prefer your host's native LSP tool (goToDefinition,
findReferences) if one is configured — then come back to the index for
repo-wide sweeps, impls, callers, unused, and impact.

With arguments, run the matching subcommand directly and act on its output.
Without arguments, run `stats` and report the index's state.
