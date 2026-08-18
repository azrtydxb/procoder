#!/bin/sh
# procoder launcher — resolves the one binary this machine runs.
#
# The plugin ships a prebuilt binary per platform under dist/ (see the design
# contract: marketplace install, no runtime, no network). This picks by OS and
# architecture and execs it with everything passed through — argv, stdin,
# stdout — so hooks.json and the skills only ever name this one path.
set -u

dir="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
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

bin="$dir/dist/$os-$arch/procoder"
if [ ! -x "$bin" ]; then
	echo "procoder: no binary for $os/$arch at $bin — reinstall the plugin" >&2
	exit 1
fi

exec "$bin" "$@"
