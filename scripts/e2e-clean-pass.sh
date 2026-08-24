#!/usr/bin/env bash
# Runs every offline procoder command against the fixture and records what
# each one said. A finding against correct code is a procoder defect; a
# command whose tool is absent is NOT RUN and is counted with neither the
# passes nor the defects.
#
# Usage: e2e-clean-pass.sh <procoder-binary> [fixture-dir]
set -uo pipefail

PC="${1:?usage: e2e-clean-pass.sh <procoder-binary> [fixture-dir]}"
PC="$(cd "$(dirname "$PC")" && pwd)/$(basename "$PC")"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
FX="${2:-${TMPDIR:-/tmp}/procoder-e2e-fixture}"
OUT="${OUT:-${TMPDIR:-/tmp}/procoder-e2e-clean}"

rm -rf "$OUT"
mkdir -p "$OUT/log"
cd "$FX" || exit 2

pass=0
finding=0
notrun=0

# run <id> <args...> — invoke procoder, record the exit code and output,
# and classify. Classification never guesses: NOT RUN is claimed only when
# the command said so in the words the no-silent-green work gave it.
run() {
	local id="$1"
	shift
	local log="$OUT/log/$id.txt"
	"$PC" "$@" >"$log" 2>&1
	local code=$?
	local verdict
	# NOT RUN is claimed only in procoder's own no-silent-green vocabulary.
	# An earlier version matched "missing" too, which reads a finding about
	# a missing PR template as a check that did not happen.
	if grep -qE 'NOT (checked|run|computed)|is not installed|could not run|no known package' "$log"; then
		verdict="NOT RUN"
		notrun=$((notrun + 1))
	elif [ "$code" -eq 0 ]; then
		verdict="PASS"
		pass=$((pass + 1))
	else
		verdict="FINDING"
		finding=$((finding + 1))
	fi
	printf '%-9s exit=%-2s %s\n' "$verdict" "$code" "procoder $*" >>"$OUT/report.txt"
	printf '%s\t%s\t%s\n' "$verdict" "$code" "$*" >>"$OUT/report.tsv"
}

: >"$OUT/report.txt"
: >"$OUT/report.tsv"

# --- the surfaces that answer about the repository -------------------
run doctor doctor
run status status
run env env
run config config
run init init
run check check
run check-tree check $(git ls-files)
run format format $(git ls-files)
run git git
run ci ci
run ci-emit ci --emit
run infra infra
run lint lint
run lint-types lint --types
run maintain maintain
run security security
run security-deep security --deep
run deps deps
run debt debt
run docs docs
run test test
run bench bench
run audit audit
run scrub scrub README.md
run principles principles
run agents agents
run templates templates
run adr-list adr list
run adr-check adr check
run lessons lessons
run version version

# --- the project layer ------------------------------------------------
run analyze-list analyze list
run backlog-list backlog list
run backlog-board backlog board
run sprint-status sprint status
run todo-list todo list
run spec-list spec list
run spec-template spec template default
run plan-list plan list
run review review
run ask ask

# --- the code index ----------------------------------------------------
run index-build index build
run index-stats index stats
run index-entrypoints index entrypoints
run index-unused index unused
run index-outline index outline greet/greet.go
run index-find index find Greet
run index-search index search Greet
run index-refs index refs Greet
run index-impls index impls greet.Greet
run index-callers index callers Greet
run index-graph index graph
run index-impact index impact greet/greet.go

printf '\n%s\n' "pass=$pass finding=$finding notrun=$notrun" >>"$OUT/report.txt"
cat "$OUT/report.txt"
echo "logs: $OUT/log"

# Every package name procoder tells a person to install must exist. This
# found `brew install rubocop`, which procoder had been printing to
# everyone missing it and which has never been a formula. Needs brew and
# the network, so it reports NOT RUN rather than passing when it cannot ask.
if ! command -v brew >/dev/null 2>&1; then
	echo "NOT RUN   brew formula names — brew is not installed here" >>"$OUT/report.txt"
else
	names=$(grep -oE '\{Manager: "brew", Args: \[\]string\{"install", "[^"]+"' \
		"$REPO/internal/tools/tools.go" | grep -oE '"[^"]+"$' | tr -d '"' | sort -u)
	# An empty list is not a clean list. The first version of this check
	# read a path relative to the fixture procoder had already cd'd into,
	# grep found nothing, the loop ran zero times, and it reported every
	# formula valid — the campaign committing the failure it hunts.
	if [ -z "$names" ]; then
		echo "NOT RUN   brew formula names — no names found in tools.go, so nothing was checked" >>"$OUT/report.txt"
	else
		bad=0
		for f in $names; do
			brew info --formula "$f" >/dev/null 2>&1 || {
				echo "FINDING   brew formula named by procoder does not exist: $f" >>"$OUT/report.txt"
				bad=$((bad + 1))
			}
		done
		[ "$bad" = 0 ] && echo "PASS      $(echo "$names" | wc -l | tr -d " ") brew formula name(s) checked, all exist" >>"$OUT/report.txt"
	fi
fi
tail -3 "$OUT/report.txt"
