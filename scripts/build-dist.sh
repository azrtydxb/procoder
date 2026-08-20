#!/usr/bin/env bash
# Rebuild dist/ — the per-platform binaries the plugin executes on every
# session start and the release publishes for download — together with the
# SHA256SUMS file that `procoder self-upgrade` verifies a download against.
#
# This exists because the procedure that produced dist/ lived only in the
# maintainer's shell history (#79). A committed binary nobody can rebuild is
# a binary nobody can check, and this repository's whole premise is checks
# done by code rather than trust.
set -euo pipefail

cd "$(dirname "$0")/.."

# The two usual sources of non-determinism, removed: a linked host libc, and
# absolute build paths baked into the binary. With them gone, and with module
# inputs already pinned by go.sum, the same Go toolchain on any machine
# produces the same bytes — which is what lets CI compare hashes against the
# committed files instead of taking the committer's word.
export CGO_ENABLED=0

# The version is part of the binary, not a label beside it: the release
# stamps it through -ldflags, `procoder version` prints it, and CI checks
# the committed binaries against the manifest. A rebuild that omitted it
# would produce binaries reporting "dev" and fail that check — so the same
# manifest CI reads is the one this script reads.
version=$(python3 -c 'import json;print(json.load(open(".claude-plugin/plugin.json"))["version"])')
if [ -z "$version" ]; then
	echo "cannot read the version from .claude-plugin/plugin.json — refusing to build unstamped binaries" >&2
	exit 1
fi
echo "building procoder $version"

# Go embeds its own version in every binary, so the toolchain is part of the
# input. Printing it makes a byte difference between two rebuilds traceable
# to a patch bump rather than to a mystery.
go version

# sha256 prints one file's digest. macOS ships shasum and no sha256sum;
# hard-coding either one makes this script run on one maintainer's laptop.
sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

sums=dist/SHA256SUMS
mkdir -p dist
: >"$sums"

for target in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64 windows-amd64; do
	os=${target%-*}
	arch=${target#*-}
	ext=""
	if [ "$os" = windows ]; then
		ext=.exe
	fi
	mkdir -p "dist/$target"
	# -buildvcs=false is what makes this reproducible at all: without it Go
	# stamps the commit hash, the commit time and whether the tree was dirty
	# into the binary, so the same source at two commits produces different
	# bytes and a hash comparison can never pass. -s -w strip the symbol and
	# DWARF tables, matching how the published binaries have always been
	# built and cutting roughly a third of the size.
	GOOS="$os" GOARCH="$arch" go build -trimpath -buildvcs=false \
		-ldflags "-s -w -X main.version=$version" \
		-o "dist/$target/procoder$ext" ./cmd/procoder
	# The names recorded here are the RELEASE ASSET names, not the dist/
	# paths. This file is published beside the assets and read by
	# self-upgrade, which knows a download only by the name it asked for; a
	# checksums file keyed on paths nobody downloads would verify nothing.
	printf '%s  procoder-%s%s\n' "$(sha256 "dist/$target/procoder$ext")" "$target" "$ext" >>"$sums"
done

cat "$sums"
