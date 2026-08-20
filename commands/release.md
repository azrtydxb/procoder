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
3. Fix everything the release controller listed — bump the files, write
   the changelog entry, commit — then rerun `launcher.sh release` until
   it answers ready.
4. When the release controller answers ready it prints the tag command;
   run that command yourself and push the tag. The binary never tags
   (P-CONTROL).
5. A repo without `[release] files` in .procoder/config.toml gets an
   honest "verified nothing" on the version-sync leg — set the list so
   the check has teeth.
