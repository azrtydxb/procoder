#!/usr/bin/env bash
# procoder pre-commit guard. Runs the same engine as the editor hook, so a
# clean local session means a clean commit.
#
# Bypass once with: git commit --no-verify
set -euo pipefail

# Lowercase: a local, not an exported setting. Override with PROCODER_BIN when
# procoder is installed globally or only available as a plugin path.
procoder="${PROCODER_BIN:-npx --no-install procoder}"

# NUL-delimited into an array: a path with a space in it used to be split into
# two paths, and procoder would exit 2 with "no such path" — or, worse, check
# neither half and say nothing about the file that was really staged.
staged=()
while IFS= read -r -d "" file; do staged+=("$file"); done \
  < <(git diff --cached --name-only --diff-filter=ACM -z)
[ ${#staged[@]} -gt 0 ] || exit 0

if ! $procoder check "${staged[@]}"; then
  echo ""
  echo "procoder: blocking commit. Fix the findings above, or run:"
  echo "  $procoder baseline <paths>     # accept pre-existing findings"
  echo "  git commit --no-verify         # bypass once"
  exit 1
fi
