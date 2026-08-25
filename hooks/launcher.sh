#!/bin/sh
# procoder launcher — resolves the one binary this machine runs, fetching it
# once if it is not here yet.
#
# The binary is NOT committed. CI builds all five targets at the tag and
# publishes them with the checksums it generated, and this script fetches the
# one for this platform on first use, verifies it, and caches it beside the
# plugin. Nothing is ever built locally — not the binaries, not a bootstrap,
# not the checksums — which is why the only thing shipped here is text.
#
# This replaces a design contract that read "marketplace install, no runtime,
# no network". The first run now needs the network. What that buys is in
# ADR 0009; what it costs is a first run that can fail, and everything below
# is about that failing safely.
#
# Two rules shape the failure handling, and only one of them is obvious:
#
#   - A HOOK that cannot get its binary warns and lets the session continue.
#     Hooks fire inside somebody's editing session; one that exits non-zero
#     takes the session with it, and procoder has then broken the thing it
#     exists to help.
#
#   - A COMMAND that cannot get its binary refuses. A launcher that exits 0
#     having run nothing is a silent green underneath every check in the
#     tool — the same defect as `check --staged` exiting 0 having assessed a
#     mistyped filename.
set -u

# ---------------------------------------------------------------- where
dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

case "$(uname -s)" in
Darwin) os=darwin ext="" ;;
Linux) os=linux ext="" ;;
MINGW* | MSYS* | CYGWIN*) os=windows ext=".exe" ;;
*)
	echo "procoder: unsupported OS $(uname -s)" >&2
	exit 1
	;;
esac

case "$(uname -m)" in
arm64 | aarch64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*)
	echo "procoder: unsupported architecture $(uname -m)" >&2
	exit 1
	;;
esac

platform="$os-$arch"
bin="$dir/dist/$platform/procoder$ext"
asset="procoder-$platform$ext"

# PROCODER_BIN is the caller's own file and NOTHING about it is checked —
# not its version, not its checksum. It exists for a mirror, for a bisect,
# and for the tests, which need to point this script at a fixture.
if [ -n "${PROCODER_BIN:-}" ]; then
	exec "$PROCODER_BIN" "$@"
fi

# ------------------------------------------------- the steady-state path
# Already here: exec it and do nothing else. This is what runs on every
# session start, every Bash call and every write, so it must stay exactly
# as cheap as it was when the binary was committed — no network, no
# subprocess, no arithmetic.
if [ -x "$bin" ]; then
	exec "$bin" "$@"
fi

# ------------------------------------------------------- failing safely
# A hook is `hook <sub>` OR anything carrying --hook. Both shapes are wired
# in hooks/claude-hooks.json: PreToolUse, PostToolUse, Stop and PreCompact
# use the first, and SessionStart uses `principles --hook`. Matching only
# the first would refuse loudly at session start on a machine that cannot
# fetch, which is the mechanism breaking sessions at the one moment it was
# written to protect them.
is_hook=no
[ "${1:-}" = "hook" ] && is_hook=yes
for a in "$@"; do
	[ "$a" = "--hook" ] && is_hook=yes
done

# give_up prints why, then obeys the split: silence and 0 for a hook,
# because no stdout is "no decision" to PreToolUse and "no context" to
# PostToolUse; a refusal for anything else.
give_up() {
	echo "procoder: $1" >&2
	if [ "$is_hook" = yes ]; then
		echo "procoder: the gate is NOT running for this action" >&2
		exit 0
	fi
	exit 1
}

# ---------------------------------------------------- the failure memory
# Hooks fire dozens of times a minute. Retrying a failing download on each
# one would put a dead network call on the hot path, so a failure is
# remembered for a short while. This is a memory, not a silence: the reason
# is still printed on every invocation.
fail_marker="$dir/dist/.fetch-failed"
fail_window=300

now="$(date +%s 2>/dev/null || echo 0)"
if [ -f "$fail_marker" ]; then
	then_at="$(head -1 "$fail_marker" 2>/dev/null || echo 0)"
	why="$(tail -n +2 "$fail_marker" 2>/dev/null)"
	case "$then_at" in
	'' | *[!0-9]*) then_at=0 ;;
	esac
	if [ "$now" -ne 0 ] && [ "$((now - then_at))" -lt "$fail_window" ]; then
		give_up "${why:-the last fetch failed} (not retrying for $((fail_window - now + then_at))s)"
	fi
fi

remember_failure() {
	mkdir -p "$dir/dist" 2>/dev/null || true
	printf '%s\n%s\n' "$now" "$1" >"$fail_marker" 2>/dev/null || true
	give_up "$1"
}

[ -n "${PROCODER_NO_FETCH:-}" ] && give_up "no binary at $bin and fetching is disabled (PROCODER_NO_FETCH)"

# ------------------------------------------------------------ the version
# Read from the manifest beside this script, never guessed. The launcher
# carries no version of its own, which is what makes it work unchanged for
# every release — and it never falls back to "latest": installing a version
# the plugin does not declare is worse than installing nothing, and it is
# the silent kind.
manifest="$dir/.claude-plugin/plugin.json"
[ -f "$manifest" ] || remember_failure "no $manifest — cannot tell which version to fetch"

version="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" | head -1)"
[ -n "$version" ] || remember_failure "no version in $manifest — cannot tell which version to fetch"

base="${PROCODER_RELEASE_BASE:-https://github.com/azrtydxb/procoder/releases/download}"
url="$base/v$version/$asset"
sums_url="$base/v$version/SHA256SUMS"

# ------------------------------------------------------------- fetching
fetch() { # fetch <url> <dest>
	if command -v curl >/dev/null 2>&1; then
		curl -sSfL --retry 2 --max-time 120 -o "$2" "$1" 2>/dev/null
	elif command -v wget >/dev/null 2>&1; then
		wget -q -T 120 -O "$2" "$1" 2>/dev/null
	else
		return 127
	fi
}

sha256() { # sha256 <file>
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		return 127
	fi
}

tmp="$(mktemp -d 2>/dev/null)" || remember_failure "could not create a temporary directory to download into"
trap 'rm -rf "$tmp"' EXIT INT TERM

fetch "$sums_url" "$tmp/SHA256SUMS" || remember_failure "could not fetch $sums_url — no network, or no release for $version"
# The platform is named, not merely implied by the URL. procoder publishes
# five assets and windows-arm64 is not among them, so somebody on that
# machine must learn that their platform is the problem rather than reading
# a 404 and guessing at the network.
fetch "$url" "$tmp/$asset" || remember_failure "no binary for $platform in release v$version (tried $url) — this platform may not be published, or the network is down"

# The checksum for THIS asset, or nothing. A manifest that downloaded but
# carries no line for this platform is a failed verification, never a pass:
# the absence of a checksum is not the absence of a problem.
want="$(awk -v a="$asset" '$2 == a || $2 == "*" a {print $1; exit}' "$tmp/SHA256SUMS")"
[ -n "$want" ] || remember_failure "SHA256SUMS from v$version carries no line for $asset — refusing to run an unverified binary"

got="$(sha256 "$tmp/$asset")" || remember_failure "no sha256sum or shasum on this machine — refusing to run an unverified binary"
[ "$got" = "$want" ] || remember_failure "checksum mismatch for $asset (expected $want, got $got) — refusing to run it"

# ------------------------------------------------------------ installing
# Rename into place, so a second launcher racing this one sees either no
# binary or a whole one. There is no third state and therefore nothing that
# can be exec'd half-written.
mkdir -p "$dir/dist/$platform" 2>/dev/null || remember_failure "could not create $dir/dist/$platform — is the plugin directory writable?"
chmod +x "$tmp/$asset" 2>/dev/null
mv -f "$tmp/$asset" "$bin" 2>/dev/null || remember_failure "could not install the binary at $bin — is the plugin directory writable?"

rm -f "$fail_marker" 2>/dev/null
[ -x "$bin" ] || remember_failure "installed $bin but it is not executable"

exec "$bin" "$@"
