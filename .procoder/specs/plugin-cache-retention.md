# Updating in place stops leaving every previous version behind

Status: complete

## Problem

`claude plugin update procoder@procoder` writes the new version into
`~/.claude/plugins/cache/procoder/procoder/<version>/` and leaves every
previous one there. Nothing removes them.

Measured on the maintainer's machine while verifying the 3.1.0 update: 55
cached versions, 1.11 GB, exactly one of them referenced by
`installed_plugins.json`. 53 carry a `dist/` directory. `claude plugin
prune` does not cover this — it removes auto-installed dependencies and
reports "nothing to prune" against a cache in that state.

An argument existed that this belongs upstream, since Claude Code's
installer creates the directories. It is settled and recorded in #181:
procoder is what accumulates the versions, procoder is what the user
updates, and waiting for somebody else's installer to grow a retention
policy is not a plan.

From 3.1.0 each cached version costs roughly a third of what it used to,
because the binaries are fetched rather than committed (ADR 0004). The
growth slowed; it did not stop, and the 1.1 GB already on disk does not
remove itself.

## Users

The maintainer and every procoder user who updates more than a few times.
The cost is invisible until somebody looks at their disk, which is why it
reached 1.1 GB before anybody did.

## In scope

- [S-1] procoder can report which cached plugin versions are removable,
  how many, and how much space they hold.
- [S-2] `procoder prune` reports and removes nothing. `procoder prune
--apply` removes. The safe behaviour is what happens when somebody types
  the command to find out what it does.
- [S-3] The version named in `installed_plugins.json` is never removed, and
  neither is the directory the running binary is executing from. Both
  checks, independently — either alone leaves a way to delete what is in
  use.
- [S-4] A retention window is kept: the active version and one previous, so
  the cheap rollback — repointing `installed_plugins.json` at an older
  directory — still exists after a sweep.
- [S-5] procoder refuses rather than guesses when `installed_plugins.json`
  cannot be read or parsed. Not knowing which version is active is not a
  licence to delete.
- [S-6] What was removed is named, and what was reclaimed is stated. A
  sweep nobody can audit is a sweep nobody trusts.
- [S-7] Nothing sweeps from a hook. This runs when a person asks for it,
  never on a path that fires on every write.

## Out of scope

- Removing other plugins' caches. procoder answers for procoder's
  directory; sweeping somebody else's is the same overreach #172 was about.
- Rewriting `installed_plugins.json`. procoder reads it and never writes
  it — that file is the installer's.
- The git-history half of the same waste, which is #180 and needs a history
  rewrite at v4.0.0.
- Reclaiming space inside a kept version. A cached version is kept whole or
  removed whole; partial directories are a state nothing else understands.

## Constraints

- **Deletion is irreversible and the default must reflect that.** Report is
  what happens without `--apply`.
- **Two independent protections on the active version**, because the
  failure is unrecoverable in a way the others are not.
- **procoder never writes `installed_plugins.json`.**
- **Refuse on doubt.** Unreadable state means stop, not proceed carefully.

## Interfaces

| Surface                             | Behaviour                                                                                                        |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `procoder prune`                    | Reports removable versions, their count and total size, and names what is kept and why. Exit 0. Removes nothing. |
| `procoder prune --apply`            | Removes them, then reports what went and how much was reclaimed.                                                 |
| Unreadable `installed_plugins.json` | Refuses, names the file and the reason, exit 2. Removes nothing under either form.                               |
| Hooks                               | Never invoke it.                                                                                                 |

## Data

Reads `~/.claude/plugins/installed_plugins.json` and the directory listing
of `~/.claude/plugins/cache/procoder/procoder/`. Writes nothing except by
removing whole version directories under `--apply`.

## Edge cases

- **The active version's directory is absent** from the cache. Report it
  and remove nothing: the state is not one this understands, and deleting
  the rest would leave no working version at all.
- **A directory name is not a version** — a partial download, an editor's
  backup. It is not a version, so the retention window cannot rank it. Kept
  and named, never removed, because guessing what an unrecognised directory
  is worth is exactly what this must not do.
- **Only the active version is cached.** Nothing to do, and it says so
  rather than printing an empty report that reads like a failure.
- **The running binary executes from a directory the window would drop** —
  possible when an older version is running. Kept, by the second check.
- **The cache directory does not exist.** Not an error. procoder may be
  installed from a release binary rather than the marketplace.
- **A version directory cannot be removed** (permissions, a file held
  open). The sweep continues, that directory is reported as not removed
  with the reason, and the reclaimed total counts only what actually went.

## Failure modes

- **The version in use is deleted**, leaving a broken install. The worst
  outcome, and the reason for two independent checks rather than one.
- **The report and the sweep disagree** — `prune` naming one set and
  `--apply` removing another. Held by both taking the same set from one
  function, so there is no second implementation to drift.
- **The reclaimed figure is invented** rather than measured, so a sweep
  that removed nothing still claims a number. Held by summing what was
  actually removed, not what was planned.
- **It gets called from a hook** by a later change, turning a deliberate
  action into one that fires on every write.

## Acceptance criteria

- [ ] [S-2] Over a fixture cache of several versions, `procoder prune`
      exits 0, names the removable set, and every directory still exists
      afterwards.
- [ ] [S-2] `procoder prune --apply` over the same fixture removes exactly
      the set the report named, and no other directory.
- [ ] [S-3] With the active version inside the window that would otherwise
      be dropped, it survives; and with the running binary's own directory
      inside that set, it survives. Asserted separately, so one check
      passing cannot hide the other being absent.
- [ ] [S-4] After a sweep of a fixture with five versions, the active
      version and exactly one previous remain.
- [ ] [S-5] With `installed_plugins.json` absent, and again with it holding
      unparseable content, `--apply` exits 2 and every directory still
      exists.
- [ ] [S-6] The report names each removed directory and states a reclaimed
      total that equals the summed size of the directories that actually
      went — verified against a fixture of known sizes, not against the
      figure the code computed.
- [ ] [S-1] A cache directory that does not exist produces a plain
      statement and exit 0, not an error.
- [ ] [S-7] No hook path reaches the sweep: asserted by searching the hook
      entry points for the call, so a later change that wires it in fails
      this test.

## Open questions

## Decisions

- **Report by default, remove behind `--apply`.** Somebody typing
  `procoder prune` to see what it does must not lose 1 GB finding out. The
  reclaim still takes one command.
- **Keep the active version and one previous.** One rollback target is the
  minimum that still counts as a rollback; more is cheap now but the
  window's job is recovery, not archival.
- **procoder deletes here, and this is the deliberate exception to
  P-CONTROL.** That rule governs repository content the agent authors — a
  plugin cache is neither. Recorded so the exception is a decision rather
  than an oversight.
