#!/usr/bin/env bash
# Removes what the campaign created, and checks it is gone rather than
# asserting it.
#
# Two things outlive their usefulness: a fixture directory full of
# deliberate secrets and vulnerable manifests, and a public repository
# holding a copy of the clean half. A command that reports success and did
# nothing is the failure this whole campaign is about, so every removal is
# followed by a look.
#
# The build script and the campaign report survive on purpose — the script
# is what makes the fixture reproducible, and the report is the result.
#
# Usage: e2e-teardown.sh [--repo <owner/name>]
set -u

REPO_SLUG=""
[ "${1:-}" = "--repo" ] && REPO_SLUG="${2:-}"
FX="${TMPDIR:-/tmp}/procoder-e2e-fixture"
HERE="$(cd "$(dirname "$0")/.." && pwd)"
pass=0
fail=0
skip=0
say() {
	printf '%-5s %s\n' "$1" "$2"
	case "$1" in PASS) pass=$((pass + 1)) ;; SKIP) skip=$((skip + 1)) ;; *) fail=$((fail + 1)) ;; esac
}

# ---- the fixture directory -------------------------------------------
rm -rf "$FX"
if [ -e "$FX" ]; then
	say FAIL "the fixture directory is still there: $FX"
else
	say PASS "the fixture directory is gone"
fi

# ---- the script still works, which is the point of keeping it --------
if "$HERE/scripts/build-e2e-fixture.sh" >/dev/null 2>&1 && [ -d "$FX/.git" ]; then
	head=$(git -C "$FX" rev-parse HEAD 2>/dev/null)
	if [ -n "$head" ]; then
		say PASS "the build script still produces a working fixture ($head)"
	else
		say FAIL "the rebuilt fixture has no commit"
	fi
else
	say FAIL "the build script no longer produces a fixture"
fi

# and remove the rebuild too, so teardown leaves nothing behind
rm -rf "$FX"
[ -e "$FX" ] && say FAIL "the rebuilt fixture survived removal" || say PASS "the rebuild is gone too"

# ---- what must survive ------------------------------------------------
for keep in "scripts/build-e2e-fixture.sh" ".procoder/analysis/e2e-campaign-report.md"; do
	if [ -f "$HERE/$keep" ]; then
		say PASS "$keep survives, as intended"
	else
		say FAIL "$keep was removed and should not have been"
	fi
done

# ---- the throwaway repository ----------------------------------------
if [ -z "$REPO_SLUG" ]; then
	say SKIP "no --repo given; the remote half was not attempted"
elif ! command -v gh >/dev/null 2>&1; then
	say SKIP "gh is not installed; the remote half was not attempted"
else
	gh repo delete "$REPO_SLUG" --yes >/dev/null 2>&1
	# The delete's own exit code is a claim. Whether the repository still
	# answers is the fact, and that is what decides the verdict.
	if gh repo view "$REPO_SLUG" >/dev/null 2>&1; then
		say FAIL "$REPO_SLUG still exists — the token needs the delete_repo scope (gh auth refresh -h github.com -s delete_repo), or delete it by hand"
	else
		say PASS "$REPO_SLUG no longer answers"
	fi
fi

printf '\npass=%s fail=%s skip=%s\n' "$pass" "$fail" "$skip"
[ "$fail" -eq 0 ]
