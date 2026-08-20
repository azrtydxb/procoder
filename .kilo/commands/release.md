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
2. Fix everything it lists — bump the files, write the changelog entry,
   commit — then rerun until it answers ready.
3. When ready it prints the tag command; run that command yourself and
   push the tag. The binary never tags (P-CONTROL).
4. A repo without `[release] files` in .procoder/config.toml gets an
   honest "verified nothing" on the version-sync leg — set the list so
   the check has teeth.
