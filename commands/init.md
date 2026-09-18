---
description: Install the formatters this repository needs, with every command visible before it runs.
---

The user invoked /procoder:init.

Use the user's explicit `--host <name>` (repeatable for additional hosts) or
`--all` selection when provided. Otherwise the binary uses reliable plugin
context. If it asks which host to set up, ask the user and re-run with their
choice; never assume all hosts or infer one from repository directories.

1. Run:

       "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" init $ARGUMENTS

   It surveys the repository and prints one install command per missing
   formatter, chosen for this machine's package managers. It updates only the
   selected hosts' local gitignore entries, leaving `.procoder/` trackable.
   It also prints the additive `.procoder/hosts.json` declaration and needed
   rule copies from the project's `AGENTS.md`. Review and write that content
   to the named paths, preserving existing integrations. If the master is
   absent, establish the project's shared contract first; do not copy unrelated
   project instructions. Plugin installation remains host-specific, not an
   init side effect. `--yes` executes formatter installs, not generated files.

2. Execute each printed command yourself, one at a time, so the user sees
   exactly what is being installed and how. If a command fails, show the error
   and stop — do not improvise a different installer; the printed command is
   the supported path.

3. For any line saying "install by hand", relay it to the user — procoder
   found no package manager it knows on this machine.

4. Finish by running:

       "${CLAUDE_PLUGIN_ROOT}/hooks/launcher.sh" doctor

   and confirm every gap is closed. A tool is installed when doctor says ok —
   an installer exiting 0 is not the proof; doctor is. Re-run
   `launcher.sh init $ARGUMENTS` with the same selection to verify the
   integration content is also current.
