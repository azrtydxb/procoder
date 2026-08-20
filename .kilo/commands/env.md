---
description: "What changed in the project's environment since you last synced: dependencies, migrations, new env keys."
---

The user invoked /procoder:env with arguments: $ARGUMENTS

The command below is the `procoder` binary on PATH.

After pulling, the repository can move under you: a lockfile changed, a
migration landed, a new environment variable appeared. Nothing fails
loudly — you just lose an afternoon to a bug that was a stale schema.

1. `procoder env` — reports what changed since the last recorded
   sync: lockfile digests per ecosystem with the install command to
   run, migrations added or removed, and new keys declared in
   `.env.example` that your `.env` lacks. Key NAMES only; no value from
   either file is ever printed.
2. Act on what it says: install, migrate, add the variable.
3. `procoder env --sync` — record the current tree as the new
   baseline. This is a statement that you HAVE installed and migrated,
   so run it after doing the work, never before.
4. Report-only by design: environment drift is judgment, never a block.
