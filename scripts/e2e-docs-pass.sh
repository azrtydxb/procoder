#!/usr/bin/env bash
# Holds docs/commands.md to what the binary does.
#
# The docs are written by hand and are correct exactly as often as somebody
# remembered, and they are the first thing an adopter reads. Three things
# are compared: the flags each entry advertises, the exit codes ADR 0003
# promises, and P-CONTROL — the binary prints, the agent writes.
#
# Which side is wrong is decided before anything changes. Fixing the binary
# to match a sentence somebody wrote is how a documentation error becomes a
# behaviour regression.
#
# Usage: e2e-docs-pass.sh <procoder-binary> [fixture-dir]
set -u

PC="${1:?usage: e2e-docs-pass.sh <procoder-binary> [fixture-dir]}"
PC="$(cd "$(dirname "$PC")" && pwd)/$(basename "$PC")"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
FX="${2:-${TMPDIR:-/tmp}/procoder-e2e-fixture}"
OUT="${OUT:-${TMPDIR:-/tmp}/procoder-e2e-docs}"

rm -rf "$OUT"
mkdir -p "$OUT"
: >"$OUT/report.txt"
pass=0
fail=0
say() {
	printf '%-5s %s\n' "$1" "$2" >>"$OUT/report.txt"
	if [ "$1" = "PASS" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); fi
}

# ---- 1. every flag the docs advertise is a flag the binary accepts -----
# The dispatch used to read flags positionally, so a flag nobody
# implemented was silently ignored rather than refused; a documented flag
# that does nothing looked exactly like one that worked.
python3 - "$REPO/docs/commands.md" >"$OUT/docflags.txt" <<'PYEOF'
import re, sys
seen = set()
for line in open(sys.argv[1]):
    m = re.match(r'^#{2,4} `procoder ([a-z-]+)([^`]*)`', line.strip())
    if not m:
        continue
    cmd, rest = m.group(1), m.group(2)
    for f in re.findall(r'--[a-z][a-z-]*', rest):
        if (cmd, f) not in seen:
            seen.add((cmd, f))
            print(cmd, f)
PYEOF

while read -r cmd flag; do
	[ -z "$cmd" ] && continue
	"$PC" "$cmd" "$flag" >/dev/null 2>"$OUT/flag.err"
	if grep -q 'is not a flag of this command' "$OUT/flag.err"; then
		say FAIL "docs advertise \`procoder $cmd $flag\` and the binary refuses it"
	else
		say PASS "\`procoder $cmd $flag\` is documented and accepted"
	fi
done <"$OUT/docflags.txt"

# The other direction: a flag the binary accepts that no entry advertises.
# `procoder review --perspectives` reads a change as analyst, architect,
# implementer and reviewer in turn, and nobody reading the docs would know
# it exists.
python3 - "$REPO/cmd/procoder/flags.go" "$OUT/docflags.txt" "$REPO/docs/commands.md" >"$OUT/undocflags.txt" <<'PYEOF'
import re, sys

src = open(sys.argv[1]).read()
# sliced rather than matched: the regex for this block was written twice and
# wrong twice, and a table it fails to find scans zero flags and reports
# every flag documented — the empty-match pass, again.
i = src.index("var knownFlags = map")
table = src[i:src.index("\n}", i)]

documented = set()
for line in open(sys.argv[2]):
    parts = line.split()
    if len(parts) == 2:
        documented.add(tuple(parts))

docs = open(sys.argv[3]).read()
found = 0
for m in re.finditer(r'"([a-z-]+)":\s*\{([^}]*)\}', table):
    cmd = m.group(1)
    for f in re.findall(r'"(--[a-z-]+)"', m.group(2)):
        found += 1
        if (cmd, f) not in documented and f not in docs:
            print(cmd, f)
if found == 0:
    print("SCAN-FAILED knownFlags")
PYEOF'
import re, sys
src = open(sys.argv[1]).read()
table = re.search(r'var knownFlags = map\[string\]\[\]string\{(.*?)
\}', src, re.S)
documented = set()
for line in open(sys.argv[2]):
    parts = line.split()
    if len(parts) == 2:
        documented.add(tuple(parts))
docs = open(sys.argv[3]).read()
for m in re.finditer(r'"([a-z-]+)":\s*\{([^}]*)\}', table.group(1) if table else ""):
    cmd = m.group(1)
    for f in re.findall(r'"(--[a-z-]+)"', m.group(2)):
        if (cmd, f) not in documented and f not in docs:
            print(cmd, f)
PYEOF

if grep -q 'SCAN-FAILED' "$OUT/undocflags.txt"; then
	say FAIL "could not read knownFlags from flags.go — this check proved nothing"
elif [ -s "$OUT/undocflags.txt" ]; then
	while read -r cmd flag; do
		[ -z "$cmd" ] && continue
		say FAIL "\`procoder $cmd $flag\` is implemented and documented nowhere"
	done <"$OUT/undocflags.txt"
else
	say PASS "every implemented flag appears in docs/commands.md"
fi

# ---- 2. the exit codes ADR 0003 promises ------------------------------
"$REPO/scripts/build-e2e-fixture.sh" >/dev/null 2>&1
cd "$FX" || exit 2

"$PC" status >/dev/null 2>&1
[ $? -eq 0 ] && say PASS "a clean read exits 0" || say FAIL "a clean read did not exit 0"

"$PC" check greet/greet.go >/dev/null 2>&1
c=$?
[ "$c" -le 1 ] && say PASS "the gate over a clean file exits 0 or 1, never 2 (got $c)" ||
	say FAIL "the gate exited $c on a clean file — 2 is reserved for usage"

printf 'package greet\n\nfunc  Bad( ) int {\nreturn 1\n}\n' >greet/bad.go
"$PC" check greet/bad.go >/dev/null 2>&1
[ $? -eq 1 ] && say PASS "findings exit 1" || say FAIL "an unformatted file did not exit 1"
rm -f greet/bad.go

"$PC" thereisnosuchcommand >/dev/null 2>&1
[ $? -eq 2 ] && say PASS "an unknown command exits 2" || say FAIL "an unknown command did not exit 2"

"$PC" check --nosuchflag >/dev/null 2>&1
[ $? -eq 2 ] && say PASS "an unknown flag exits 2" || say FAIL "an unknown flag did not exit 2"

"$PC" adr >/dev/null 2>&1
[ $? -eq 2 ] && say PASS "a missing subcommand exits 2" || say FAIL "a missing subcommand did not exit 2"

# ---- 3. P-CONTROL: the binary prints, the agent writes ----------------
# Every read-only command runs against the fixture and the tree is
# digested before and after. `procoder format` is the sharpest case: it
# prints a formatted file and must not write one.
digest() {
	find . -path ./.git -prune -o -path ./node_modules -prune -o -type f -print0 |
		sort -z | xargs -0 shasum 2>/dev/null | shasum | cut -d' ' -f1
}

# `bench` and `release` are in the list because they are the two commands
# that legitimately CAN write — bench saves a baseline under --save, and
# release exists to prepare a tag. Without the flag and without readiness
# neither may touch anything, and they were the obvious P-CONTROL targets
# that the loop did not cover.

# An UNFORMATTED file, planted before the loop. Running `procoder format`
# only on files that are already clean tests nothing: the unformatted
# branch — the one that prints a whole rewritten file — never executes, so
# a mutation that made it write to disk passed the P-CONTROL check
# untouched. This is the case P-CONTROL exists for.
printf 'package greet\n\nfunc  Untidy( ) int {\nreturn 1\n}\n' >greet/untidy.go

for cmd in "status" "check" "git" "ci" "lint" "maintain" "security" "debt" "docs" \
	"config" "doctor" "init" "principles" "agents" "templates" "lessons" "review" \
	"backlog board" "sprint status" "todo list" "spec list" "plan list" "analyze list" \
	"format greet/greet.go" "format main.go" "format greet/untidy.go" \
	"check greet/untidy.go" "lint greet/untidy.go" \
	"bench" "release 0.9.9"; do
	before=$(digest)
	# shellcheck disable=SC2086
	"$PC" $cmd >/dev/null 2>&1
	after=$(digest)
	if [ -z "$before" ]; then
		say FAIL "could not digest the tree — the P-CONTROL check proved nothing for '$cmd'"
	elif [ "$before" = "$after" ]; then
		say PASS "\`procoder $cmd\` wrote nothing (P-CONTROL)"
	else
		say FAIL "\`procoder $cmd\` CHANGED the tree"
	fi
done

rm -f greet/untidy.go

cd "$REPO" || exit 2
printf '\npass=%s fail=%s\n' "$pass" "$fail" >>"$OUT/report.txt"
cat "$OUT/report.txt"
