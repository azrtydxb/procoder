---
description: "The canonical linter per ecosystem over your changes: findings are diagnoses you judge, fix, or explain."
---

The user invoked /procoder:lint with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

1. Run `launcher.sh lint` (or `launcher.sh lint <paths>` with arguments).
   Each finding is `file:line message (lint)` from the ecosystem's canonical
   linter — golangci-lint, ruff check, shellcheck, eslint — under the
   project's own configuration.
2. Judge every finding honestly: fix what is real, and say why for anything
   you leave (a false positive, a deliberate choice). Never silence a finding
   by deleting the code's intent.
3. Lines saying NOT checked mean a linter is missing — run `launcher.sh init`.
   Configless plain JavaScript gets the procoder baseline (eslint's
   built-in core rules, labeled "procoder baseline") — the project's own
   config always wins when present. Configless TypeScript stays out of
   scope: its parser is not built into eslint, and procoder installs no
   packages into repos.
4. Findings are informational by default; the repo can make them block the
   gate with `[lint] policy = "block"` in .procoder/config.toml.
5. Re-run after fixing and show the user the result.
