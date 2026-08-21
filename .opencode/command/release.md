---
description: "The pre-tag controller: version sync, changelog, clean tree, gate, and suite — every failure listed, the tag printed, never run."
---

The user invoked /procoder:release with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

1. Run `procoder release <version>` (or bare `procoder release`
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
   the changelog entry, commit — then rerun `procoder release` until
   it answers ready. The entry's layout is not free-form: CHANGELOG.md
   opens with the template, and the release job publishes the entry
   verbatim as the GitHub Release notes, so what you write there is what
   someone downloading the binary reads. A one-sentence italic summary
   first, then headlines that open with their kind — Added, Fixed,
   Changed, Removed, Security — each linking its issue, worst breakage
   first. A test over the newest entry holds the shape.
5. When the release controller answers ready it prints the tag command;
   run that command yourself and push the tag. The binary never tags
   (P-CONTROL).
6. A repo without `[release] files` in .procoder/config.toml gets an
   honest "verified nothing" on the version-sync leg — set the list so
   the check has teeth.
