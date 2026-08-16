#!/usr/bin/env bash
# procoder — statusline badge. Prints nothing when procoder is inactive.
set -uo pipefail

config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
level_file="$config_dir/.procoder-active"

[ -r "$level_file" ] || exit 0

level="$(tr -d '[:space:]' < "$level_file" | tr '[:upper:]' '[:lower:]')"

case "$level" in
  pragmatic|strict|paranoid) printf '[PROCODER:%s]\n' "$(printf '%s' "$level" | tr '[:lower:]' '[:upper:]')" ;;
  *) exit 0 ;;
esac
