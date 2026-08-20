---
description: "The pre-tag controller: version sync, changelog, clean tree, gate, and suite — every failure listed, the tag printed, never run."
---

The user invoked /procoder:release with arguments: $ARGUMENTS

The launcher is: "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh"

1. Run `launcher.sh release <version>` (or bare `launcher.sh release`
   to check the newest changelog version). The controller verifies in
   one pass: every file in `[release] files` carries the version, the
   changelog has its entry, the working tree is clean, the gate is
   clean, and the suite is green under [test] policy.
2. Before the first fix, run /procoder:simplify over the whole repo
   (`repo`): take the cuts you agree with, say why not for the rest.
3. Rebuild the committed binaries with `scripts/build-dist.sh` whenever
   this release changes the binary. It writes `dist/` and
   `dist/SHA256SUMS` together, which is the point: the workflow publishes
   that file beside the assets, `self-upgrade` verifies every download
   against it, and a release whose checksums describe the previous build
   is one nobody can upgrade to. A test compares the file against the
   committed binaries, so a hand-built `dist/` goes red before the tag.
4. Fix everything the release controller listed — bump the files, write
   the changelog entry, commit — then rerun `launcher.sh release` until
   it answers ready.
5. When the release controller answers ready it prints the tag command;
   run that command yourself and push the tag. The binary never tags
   (P-CONTROL).
6. A repo without `[release] files` in .procoder/config.toml gets an
   honest "verified nothing" on the version-sync leg — set the list so
   the check has teeth.
