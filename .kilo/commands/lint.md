---
description: "The canonical linter per ecosystem over your changes: findings are diagnoses you judge, fix, or explain."
---

The user invoked /procoder:lint with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

1. Run `procoder lint` (or `procoder lint <paths>` with arguments).
   Each finding is `file:line message (lint)` from the ecosystem's canonical
   linter — golangci-lint, ruff check, shellcheck, eslint — under the
   project's own configuration.
2. Judge every finding honestly: fix what is real, and say why for anything
   you leave (a false positive, a deliberate choice). Never silence a finding
   by deleting the code's intent.
3. Lines saying NOT checked mean a linter is missing — run `procoder init`.
   Configless plain JavaScript gets the procoder baseline (eslint's
   built-in core rules, labeled "procoder baseline") — the project's own
   config always wins when present. Configless TypeScript stays out of
   scope: its parser is not built into eslint, and procoder installs no
   packages into repos.
4. Findings are informational by default; the repo can make them block the
   gate with `[lint] policy = "block"` in .procoder/config.toml.
5. `procoder lint --types [paths]` adds the type-checker where the
   linter does not compile the code: `tsc --noEmit` for TypeScript (under
   the project's own tsconfig) and pyright for Python. Go and Rust need
   no flag — their linters already compile. Use it after refactors and
   renames to catch type fallout the linters cannot see.
6. Re-run after fixing and show the user the result.
