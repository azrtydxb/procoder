#!/usr/bin/env bash
# Feeds each registered hook a real payload on stdin and parses what comes
# back the way the host parses it.
#
# The suite calls these functions; the host runs this process. A hook that
# returns the right verdict in a unit test and the wrong envelope on stdout
# is broken everywhere it matters, and nothing else procoder ships is
# tested at the process boundary at all.
#
# Malformed input is part of the contract: empty stdin and a truncated
# payload are what a host sends when something upstream went wrong, and a
# hook that crashes on those takes the session with it.
#
# Usage: e2e-hook-pass.sh <procoder-binary> [fixture-dir]
set -u

PC="${1:?usage: e2e-hook-pass.sh <procoder-binary> [fixture-dir]}"
PC="$(cd "$(dirname "$PC")" && pwd)/$(basename "$PC")"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
FX="${2:-${TMPDIR:-/tmp}/procoder-e2e-fixture}"
OUT="${OUT:-${TMPDIR:-/tmp}/procoder-e2e-hooks}"

rm -rf "$OUT"
mkdir -p "$OUT"
pass=0
fail=0

say() {
	printf '%-5s %s\n' "$1" "$2" >>"$OUT/report.txt"
	if [ "$1" = "PASS" ]; then pass=$((pass + 1)); else fail=$((fail + 1)); fi
}

# feed <name> <payload> <args...> — run the hook with payload on stdin,
# leaving stdout in $OUT/<name>.out, stderr in .err and the code in .code
feed() {
	local name="$1" payload="$2"
	shift 2
	printf '%s' "$payload" | "$PC" "$@" >"$OUT/$name.out" 2>"$OUT/$name.err"
	echo $? >"$OUT/$name.code"
}

# json_ok <file> — the file parses as one JSON document
json_ok() { python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$1" 2>/dev/null; }

# field <file> <dotted.path> — print one value out of the hook envelope, or
# nothing. Reading the decision beats grepping for a word: "block" appears
# inside "1 blocking finding(s)" whatever the decision was, so a grep for it
# passes on an allow just as happily as on a deny.
field() {
	python3 - "$1" "$2" <<'PYEOF' 2>/dev/null
import json, sys
try:
    v = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
for k in sys.argv[2].split("."):
    if not isinstance(v, dict) or k not in v:
        sys.exit(0)
    v = v[k]
print(v if isinstance(v, str) else json.dumps(v))
PYEOF
}

: >"$OUT/report.txt"
"$REPO/scripts/build-e2e-fixture.sh" >/dev/null 2>&1
cd "$FX" || exit 2

CWD="$PWD"

# ---------------------------------------------------------- PostToolUse
# The host sends this after a Write or Edit. procoder's half is to answer
# about the file that was just written.
printf 'package greet\n\nfunc  Bad( ) int {\nreturn 1\n}\n' >greet/bad.go
PTU=$(printf '{"session_id":"e2e","transcript_path":"/dev/null","cwd":"%s","hook_event_name":"PostToolUse","tool_name":"Write","tool_input":{"file_path":"%s/greet/bad.go"},"tool_response":{"success":true}}' "$CWD" "$CWD")
feed posttooluse "$PTU" hook post-tool-use

if [ -s "$OUT/posttooluse.out" ] || [ -s "$OUT/posttooluse.err" ]; then
	say PASS "PostToolUse answers about the file it was handed"
else
	say FAIL "PostToolUse said nothing about an unformatted file"
fi
if grep -q 'bad.go' "$OUT/posttooluse.out" "$OUT/posttooluse.err" 2>/dev/null; then
	say PASS "PostToolUse names the file"
else
	say FAIL "PostToolUse output does not name the file it was given"
fi
ctx=$(field "$OUT/posttooluse.out" hookSpecificOutput.additionalContext)
if [ -n "$ctx" ]; then
	say PASS "PostToolUse returns additionalContext the host injects"
else
	say FAIL "PostToolUse envelope carries no additionalContext"
fi
case "$ctx" in
*"NOT modified"*) say PASS "the context tells the agent the file was left alone" ;;
*) say FAIL "the context does not say the file was left unmodified" ;;
esac
# P-CONTROL: the binary prints, the agent writes
if grep -q 'func  Bad( ) int' greet/bad.go; then
	say PASS "PostToolUse left the file unmodified (P-CONTROL)"
else
	say FAIL "PostToolUse MODIFIED the file it was asked about"
fi
rm -f greet/bad.go

# ---------------------------------------------------------- PreToolUse
# The host sends this before a Bash call. procoder's half is to stop a
# commit the gate would block.
printf 'package greet\n\nfunc  Worse( ) int {\nreturn 1\n}\n' >greet/worse.go
PRE=$(printf '{"session_id":"e2e","cwd":"%s","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git commit -m \\"feat: a thing\\"","description":"commit"}}' "$CWD")
feed pretooluse "$PRE" hook pre-tool-use
code=$(cat "$OUT/pretooluse.code")

# The host reads either exit 2 with a reason on stderr, or a JSON envelope
# carrying a deny decision. Both are valid; silence with exit 0 is not.
decision=$(field "$OUT/pretooluse.out" hookSpecificOutput.permissionDecision)
if [ "$decision" = "deny" ] || [ "$code" = "2" ]; then
	say PASS "PreToolUse denies a commit the gate would block (decision=${decision:-none}, exit $code)"
else
	say FAIL "PreToolUse did not deny a commit with an unformatted file (decision=${decision:-none}, exit $code)"
fi
reason=$(field "$OUT/pretooluse.out" hookSpecificOutput.permissionDecisionReason)
case "$reason" in
*worse.go*) say PASS "the denial names the file that caused it" ;;
*) say FAIL "the denial gives the agent nothing to act on: ${reason:-<empty>}" ;;
esac
event=$(field "$OUT/pretooluse.out" hookSpecificOutput.hookEventName)
if [ "$event" = "PreToolUse" ]; then
	say PASS "the envelope names its own event, as the host requires"
else
	say FAIL "hookEventName is ${event:-missing}, not PreToolUse"
fi
if [ -s "$OUT/pretooluse.out" ] && json_ok "$OUT/pretooluse.out"; then
	say PASS "PreToolUse stdout is a JSON document the host can parse"
elif [ -s "$OUT/pretooluse.out" ]; then
	say FAIL "PreToolUse wrote non-JSON to stdout, where the host expects a decision envelope"
else
	say PASS "PreToolUse uses the exit-code path and leaves stdout empty"
fi

# a Bash call that is NOT a commit must pass straight through
NOT=$(printf '{"cwd":"%s","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls -la"}}' "$CWD")
feed pretooluse_ls "$NOT" hook pre-tool-use
lsdecision=$(field "$OUT/pretooluse_ls.out" hookSpecificOutput.permissionDecision)
if [ "$(cat "$OUT/pretooluse_ls.code")" = "0" ] && [ -z "$lsdecision" ]; then
	say PASS "PreToolUse lets a non-commit Bash call through with no decision at all"
else
	say FAIL "PreToolUse ruled on 'ls -la' (decision=${lsdecision:-none}, exit $(cat "$OUT/pretooluse_ls.code"))"
fi
rm -f greet/worse.go

# ---------------------------------------------------------- SessionStart
feed sessionstart '{"hook_event_name":"SessionStart","source":"startup"}' principles --hook
if [ -s "$OUT/sessionstart.out" ]; then
	say PASS "SessionStart returns the principles for the session"
else
	say FAIL "SessionStart produced no output — the session starts ungoverned"
fi
if [ "$(cat "$OUT/sessionstart.code")" = "0" ]; then
	say PASS "SessionStart exits 0"
else
	say FAIL "SessionStart exited $(cat "$OUT/sessionstart.code"); a non-zero here is a startup error"
fi

# ---------------------------------------------------------- Stop
feed stop '{"hook_event_name":"Stop","stop_hook_active":false}' hook stop
if [ "$(cat "$OUT/stop.code")" = "0" ]; then
	say PASS "Stop exits 0"
else
	say FAIL "Stop exited $(cat "$OUT/stop.code")"
fi

# ------------------------------------------------- malformed input
# A host that failed upstream sends nothing, or half a payload. Neither may
# take the session with it.
for h in "hook post-tool-use" "hook pre-tool-use" "hook stop"; do
	id=$(echo "$h" | tr ' ' '_')
	feed "${id}_empty" "" $h
	c=$(cat "$OUT/${id}_empty.code")
	if [ "$c" -gt 2 ] || grep -qi 'panic\|goroutine ' "$OUT/${id}_empty.err" 2>/dev/null; then
		say FAIL "$h crashed on empty stdin (exit $c)"
	else
		say PASS "$h survives empty stdin (exit $c)"
	fi

	feed "${id}_trunc" '{"tool_name":"Write","tool_inp' $h
	c=$(cat "$OUT/${id}_trunc.code")
	if [ "$c" -gt 2 ] || grep -qi 'panic\|goroutine ' "$OUT/${id}_trunc.err" 2>/dev/null; then
		say FAIL "$h crashed on a truncated payload (exit $c)"
	else
		say PASS "$h survives a truncated payload (exit $c)"
	fi
done

# a payload naming a file that is not there
GONE=$(printf '{"cwd":"%s","hook_event_name":"PostToolUse","tool_name":"Write","tool_input":{"file_path":"%s/does/not/exist.go"}}' "$CWD" "$CWD")
feed posttooluse_gone "$GONE" hook post-tool-use
c=$(cat "$OUT/posttooluse_gone.code")
if [ "$c" -gt 2 ] || grep -qi 'panic\|goroutine ' "$OUT/posttooluse_gone.err" 2>/dev/null; then
	say FAIL "PostToolUse crashed on a file that does not exist (exit $c)"
else
	say PASS "PostToolUse survives a file that does not exist (exit $c)"
fi

cd "$REPO" || exit 2
printf '\npass=%s fail=%s\n' "$pass" "$fail" >>"$OUT/report.txt"
cat "$OUT/report.txt"
