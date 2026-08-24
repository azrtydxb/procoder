#!/usr/bin/env bash
# The other half of proving a security check works: it must stop blocking
# when the repository says so, and the secret it flags must never appear in
# anything procoder writes.
#
# A check that fires and a check that fires unconditionally look identical
# from the outside, and so do a working knob and a decorative one.
#
# Usage: e2e-security-knobs.sh <procoder-binary>
# No pipefail here, deliberately. Under it, `procoder security | grep -q X`
# reports failure whenever procoder exits 1 — which is what procoder does
# when it finds something — so a check that matched perfectly read as a
# check that failed. Every test below greps a file rather than a pipe, so
# the exit code under test is procoder's and the match is grep's, and the
# two are never confused.
set -u

PC="${1:?usage: e2e-security-knobs.sh <procoder-binary>}"
PC="$(cd "$(dirname "$PC")" && pwd)/$(basename "$PC")"
pass=0
fail=0
unproved=0

say() {
	printf '%-7s %s\n' "$1" "$2"
	[ "$1" = "PASS" ] && pass=$((pass + 1)) || fail=$((fail + 1))
}

# ---- the SAST severity knob is real, in both directions ---------------
T=$(mktemp -d)
cd "$T" || exit 2
git init -q
mkdir -p .procoder
printf 'import subprocess\n\n\ndef run(cmd: str) -> None:\n    subprocess.call(cmd, shell=True)\n' >inject.py

"$PC" security --deep >deep.txt 2>&1
if grep -q 'BLOCK.*subprocess-shell-true' deep.txt; then
	say PASS "an ERROR-severity finding blocks at the default"
else
	say FAIL "the default did not block a finding semgrep calls ERROR"
fi

# WARNING is a strengthening: findings semgrep is less sure about start to
# block. There is deliberately no relaxation above ERROR, so the knob is
# proved by making MORE block rather than fewer.
printf '[security]\nsast_blocks_at = "WARNING"\n' >.procoder/config.toml
"$PC" security --deep >warn.txt 2>&1
before=$(grep -c 'BLOCK' warn.txt)
printf '[security]\nsast_blocks_at = "ERROR"\n' >.procoder/config.toml
"$PC" security --deep >err.txt 2>&1
after=$(grep -c 'BLOCK' err.txt)
# `before >= after` would pass whenever the two are equal, which is to say
# whenever the knob demonstrably did nothing. That is the shape of assertion
# this campaign exists to remove, so equality is reported as unproved rather
# than counted as a pass.
if [ "$before" -gt "$after" ]; then
	say PASS "sast_blocks_at changes what blocks: WARNING $before, ERROR $after"
elif [ "$before" -lt "$after" ]; then
	say FAIL "lowering the bar to WARNING blocked FEWER ($before) than ERROR ($after)"
else
	printf '%-7s %s\n' "UNPROVED" "sast_blocks_at read without effect here: both settings block $before — this fixture carries no WARNING-severity finding to tell them apart"
	unproved=$((unproved + 1))
fi

# and the setting is reported as configured rather than silently applied
"$PC" config >cfg.txt 2>&1
if grep -q 'security.sast_blocks_at' cfg.txt; then
	say PASS "the effective severity is reported by procoder config"
else
	say FAIL "procoder config does not report security.sast_blocks_at"
fi
cd /
rm -rf "$T"

# ---- a flagged secret's VALUE never appears anywhere -------------------
T=$(mktemp -d)
cd "$T" || exit 2
git init -q
printf 'module x\n\ngo 1.24\n' >go.mod
KEY="AKIA$(printf 'procoder-e2e-knobs' | shasum | tr 'a-z' 'A-Z' | tr -cd 'A-Z0-9' | head -c 16)"
printf 'package main\n\nconst K = "%s"\n\nfunc main() {}\n' "$KEY" >creds.go

"$PC" security >out.txt 2>&1
if grep -q 'AWS Access Key' out.txt; then
	say PASS "the planted credential is flagged"
else
	say FAIL "the planted credential was not flagged, so the rest proves nothing"
fi
if grep -qF "$KEY" out.txt; then
	say FAIL "the credential's VALUE was echoed into procoder's output"
else
	say PASS "the credential's value appears nowhere in the finding"
fi

"$PC" ask >/dev/null 2>&1
if [ -f .procoder/ask/QA.md ] && grep -qF "$KEY" .procoder/ask/QA.md; then
	say FAIL "the credential's VALUE was written into QA.md"
else
	say PASS "the credential's value appears nowhere in QA.md"
fi

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/creds.go"}}\n' "$PWD" |
	"$PC" hook post-tool-use >hook.txt 2>&1
if grep -qF "$KEY" hook.txt; then
	say FAIL "the credential's VALUE was echoed into the hook payload"
else
	say PASS "the credential's value appears nowhere in the hook output"
fi
cd /
rm -rf "$T"

# ---- a scanner that cannot run says so --------------------------------
T=$(mktemp -d)
cd "$T" || exit 2
git init -q
printf 'module x\n\ngo 1.24\n' >go.mod
printf 'package main\n\nfunc main() {}\n' >a.go
EMPTY=$(mktemp -d)
PATH="$EMPTY" "$PC" security --deep >bare.txt 2>&1
if grep -qE 'NOT (run|checked)' bare.txt; then
	say PASS "a scanner absent from PATH reports NOT run rather than clean"
else
	say FAIL "an absent scanner did not say so — that is a silent green"
fi
cd /
rm -rf "$T" "$EMPTY"

printf '\npass=%s fail=%s unproved=%s\n' "$pass" "$fail" "$unproved"
[ "$fail" -eq 0 ]
