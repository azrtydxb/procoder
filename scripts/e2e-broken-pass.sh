#!/usr/bin/env bash
# Plants one deliberate defect per class procoder claims to catch, and
# checks that each is caught by the command that owns it.
#
# Every defect is planted alone, in a fresh fixture, and removed before the
# next: two defects at once cannot tell you which one was found, and a
# check that catches nine of ten looks identical to one that catches ten
# when they are planted together.
#
# CAUGHT means the owning command reported it AND named the file. MISSED is
# the finding this pass exists to produce.
#
# Usage: e2e-broken-pass.sh <procoder-binary> [fixture-dir]
set -uo pipefail

PC="${1:?usage: e2e-broken-pass.sh <procoder-binary> [fixture-dir]}"
PC="$(cd "$(dirname "$PC")" && pwd)/$(basename "$PC")"
REPO="$(cd "$(dirname "$0")/.." && pwd)"
FX="${2:-${TMPDIR:-/tmp}/procoder-e2e-fixture}"
OUT="${OUT:-${TMPDIR:-/tmp}/procoder-e2e-broken}"

rm -rf "$OUT"
mkdir -p "$OUT/log"
: >"$OUT/report.txt"

caught=0
missed=0

# plant <id> <expect-file> <command...> — rebuild the fixture, run the
# planting function named plant_<id>, then run the command and require the
# output to name the file.
plant() {
	local id="$1" expect="$2"
	shift 2
	"$REPO/scripts/build-e2e-fixture.sh" >/dev/null 2>&1
	cd "$FX" || exit 2
	"plant_$id"
	local log="$OUT/log/$id.txt"
	"$PC" "$@" >"$log" 2>&1
	local code=$?
	if grep -qF "$expect" "$log"; then
		printf 'CAUGHT  %-26s exit=%-2s %s\n' "$id" "$code" "procoder $*" >>"$OUT/report.txt"
		caught=$((caught + 1))
	else
		printf 'MISSED  %-26s exit=%-2s %s\n' "$id" "$code" "procoder $*" >>"$OUT/report.txt"
		missed=$((missed + 1))
	fi
	cd "$REPO" || exit 2
}

# ---- formatting, one language at a time -----------------------------
plant_fmt_go() { printf 'package greet\n\nfunc  Sloppy( ) string {\nreturn "x"\n}\n' >greet/sloppy.go; }
plant_fmt_py() { printf 'def sloppy( a,b ):\n      return a+b\n' >py/sloppy.py; }
plant_fmt_rs() { printf 'pub fn sloppy(  ) -> i32 {\n1\n}\n' >src/sloppy.rs; }
plant_fmt_c() { printf '#include <stdio.h>\nint sloppy(  int a ,int b){return a+b;}\n' >c/sloppy.c; }
plant_fmt_sh() { printf '#!/usr/bin/env bash\nif [ 1 = 1 ]\n  then\n  echo hi\nfi\n' >sh/sloppy.sh; }
plant_fmt_java() { printf 'package invalid.example;\npublic class Sloppy{public static int f(  int a){return a;}}\n' >src/main/java/invalid/example/Sloppy.java; }
plant_fmt_kt() { printf 'package invalid.example\nfun sloppy(  a:Int ):Int{return a}\n' >kt/Sloppy.kt; }
plant_fmt_swift() { printf 'public func sloppy(  a:Int )->Int{return a}\n' >swift/Sloppy.swift; }
plant_fmt_rb() { printf '# frozen_string_literal: true\ndef sloppy( a,b )\n    a+b\nend\n' >rb/sloppy.rb; }
plant_fmt_dart() { printf 'int sloppy(  int a ){return a;}\n' >dart/sloppy.dart; }
plant_fmt_cs() { printf 'namespace Fixture;\npublic static class Sloppy{public static int F(  int a){return a;}}\n' >cs/Sloppy.cs; }
plant_fmt_php() { printf '<?php\nfunction sloppy(  $a ){return $a;}\n' >php/Sloppy.php; }
plant_fmt_web() { printf 'export function sloppy(  a ){return a}\n' >web/sloppy.js; }

# ---- everything else -------------------------------------------------
plant_lint() { printf '#!/usr/bin/env bash\nset -euo pipefail\nunused_var=1\necho "$HOME"\n' >sh/unused.sh; }
plant_secret() {
	printf 'package greet\n\n// nolint\nconst AWSKey = "AKIAIOSFODNN7EXAMPLE"\nconst AWSSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"\n' >greet/creds.go
}
plant_sast() {
	printf 'import subprocess\n\n\ndef run(cmd: str) -> None:\n    subprocess.call(cmd, shell=True)\n' >py/shellinject.py
}
plant_vulndep() {
	printf 'module example.invalid/fixture\n\ngo 1.24\n\nrequire golang.org/x/text v0.3.0\n' >go.mod
}
plant_conflict() {
	printf 'package greet\n\n<<<<<<< HEAD\nconst A = 1\n=======\nconst A = 2\n>>>>>>> other\n' >greet/conflict.go
}
plant_oversized() { dd if=/dev/zero of=big.bin bs=1m count=12 >/dev/null 2>&1; }
plant_debt() { printf 'package greet\n\n// debt: global lock here, it is O(n2)\nconst Slow = 1\n' >greet/slow.go; }
plant_docref() { printf '\nSee [the missing page](docs/nowhere.md) for details.\n' >>README.md; }
plant_attribution() { :; }

# ---- run -------------------------------------------------------------
for lang in go py rs c sh java kt swift rb dart cs php web; do
	case "$lang" in
	go) f=greet/sloppy.go ;; py) f=py/sloppy.py ;; rs) f=src/sloppy.rs ;;
	c) f=c/sloppy.c ;; sh) f=sh/sloppy.sh ;; java) f=src/main/java/invalid/example/Sloppy.java ;;
	kt) f=kt/Sloppy.kt ;; swift) f=swift/Sloppy.swift ;; rb) f=rb/sloppy.rb ;;
	dart) f=dart/sloppy.dart ;; cs) f=cs/Sloppy.cs ;; php) f=php/Sloppy.php ;;
	web) f=web/sloppy.js ;;
	esac
	plant "fmt_$lang" "$f" check "$f"
done

plant lint sh/unused.sh lint sh/unused.sh
plant secret greet/creds.go security greet/creds.go
plant sast py/shellinject.py security --deep
plant vulndep golang.org/x/text security --deep
plant conflict greet/conflict.go check greet/conflict.go
plant oversized big.bin check big.bin
plant debt greet/slow.go debt
plant docref docs/nowhere.md docs

# AI attribution is checked in a commit message, not a file
"$REPO/scripts/build-e2e-fixture.sh" >/dev/null 2>&1
cd "$FX" || exit 2
printf 'feat: a thing\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n' >/tmp/e2e-msg.txt
if "$PC" scrub /tmp/e2e-msg.txt >"$OUT/log/attribution.txt" 2>&1; then
	printf 'MISSED  %-26s exit=0  procoder scrub <message>\n' attribution >>"$OUT/report.txt"
	missed=$((missed + 1))
else
	printf 'CAUGHT  %-26s exit=1  procoder scrub <message>\n' attribution >>"$OUT/report.txt"
	caught=$((caught + 1))
fi
cd "$REPO" || exit 2

printf '\ncaught=%s missed=%s\n' "$caught" "$missed" >>"$OUT/report.txt"
cat "$OUT/report.txt"
echo "logs: $OUT/log"
