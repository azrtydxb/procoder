#!/usr/bin/env bash
# procoder pre-commit guard. Runs the same engine as the editor hook, so a
# clean local session means a clean commit.
#
# Bypass once with: git commit --no-verify
set -euo pipefail

# Lowercase: a local, not an exported setting. Override with PROCODER_BIN when
# procoder is installed globally or only available as a plugin path.
procoder="${PROCODER_BIN:-npx --no-install procoder}"

staged="$(git diff --cached --name-only --diff-filter=ACM)"
[ -n "$staged" ] || exit 0

# shellcheck disable=SC2086
if ! $procoder check $staged; then
  echo ""
  echo "procoder: blocking commit. Fix the findings above, or run:"
  echo "  $procoder baseline $staged     # accept pre-existing findings"
  echo "  git commit --no-verify          # bypass once"
  exit 1
fi
