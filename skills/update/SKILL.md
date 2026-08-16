---
name: update
description: Update an installed procoder to the latest version, and name what the update breaks. Use when the user says "update procoder", "upgrade procoder", "get the latest version of procoder", "is procoder up to date", or invokes /procoder:update.
---

# procoder:update

A plain `git pull` updates the code and silently invalidates two things it does
not touch: every `.procoder-baseline.json` if the baseline format changed, and
every hand-copied rule file on other platforms if the doctrine changed. Report
both, or the user meets them in CI.

## 1. Find the install

Let `CFG` be `$CLAUDE_CONFIG_DIR`, else `~/.claude`. Check in order, take the
first that has `.claude-plugin/plugin.json`:

| Candidate | Kind |
|---|---|
| `$CFG/plugins/marketplaces/procoder` | marketplace |
| `$CFG/plugins/cache/procoder` | marketplace cache |
| the current repo, if its `package.json` name is `procoder` | local clone |
| a path the user names | local clone |

Say which one you found and its absolute path. If none matches, say plainly that
no install was found, name the directories you checked, and stop — do not clone one
uninvited.

## 2. Compare versions, before running anything

```bash
git -C <dir> rev-parse --is-inside-work-tree      # git-managed?
git -C <dir> status --porcelain                   # local modifications?
git -C <dir> fetch --quiet origin
git -C <dir> show HEAD:.claude-plugin/plugin.json      # installed version
git -C <dir> show origin/HEAD:.claude-plugin/plugin.json  # latest version
```

Print: directory, kind, installed version, latest version, and the exact command
you will run. Then act.

- **Same version** — say already up to date and stop. Change nothing.
- **`git status --porcelain` is non-empty** — list the modified paths and stop.
  They are the user's edits; ask before going further.
- **Not a git work tree** — say so and point at `docs/install.md`; the `claude`
  CLI is often not on `PATH`, and `/plugin` does not exist in a
  non-interactive session, so there is no reliable path from here.

## 3. Update

```bash
git -C <dir> merge --ff-only origin/HEAD
```

Report the command and its output verbatim. If the fast-forward is refused, stop
and report — never force the branch into shape.

## 4. Report what changed

- **CHANGELOG.md** — `git -C <dir> diff <old>..<new> -- CHANGELOG.md`. Summarise
  in at most five lines. Do not paste the file.
- **Baseline format.** Compare `BASELINE_VERSION` in `hooks/checks/baseline.js`
  across the two revisions — read the constant, never assume its value:

  ```bash
  git -C <dir> show <old>:hooks/checks/baseline.js | grep BASELINE_VERSION
  git -C <dir> show <new>:hooks/checks/baseline.js | grep BASELINE_VERSION
  ```

  Changed? Say so first, in its own block: every existing
  `.procoder-baseline.json` stops matching, `procoder verify` exits 2 with
  "cannot verify, re-baseline required", and `procoder check` surfaces the
  repo's whole historical backlog. The fix is one command, run per repo:
  `procoder baseline <paths>`.
- **Doctrine.** `git -C <dir> diff <old>..<new> -- skills/procoder/SKILL.md`.
  Non-empty means every hand-copied rule file on another platform is now behind.
  List the ones to re-copy from the install directory, per `docs/install.md`:

  | Platform | Re-copy |
  |---|---|
  | Cursor | `.cursor/rules/procoder.mdc` |
  | Windsurf | `.windsurf/rules/procoder.md` |
  | Cline | `.clinerules/procoder.md` |
  | Kiro | `.kiro/steering/procoder.md` |
  | Qoder | `.qoder/rules/procoder.md` |
  | AGENTS.md hosts, opencode, openclaw | `AGENTS.md`, `.opencode/command/*.md`, `.openclaw/` |

  Empty diff? Say the copies are still current — silence reads as "unchecked".

## Do not

- Do not run any update command before printing the directory and version delta.
- Do not `git reset --hard`, `git checkout --`, `stash`, or otherwise discard or
  overwrite local modifications — report them and stop.
- Do not `git pull --rebase` or force a merge; fast-forward only.
- Do not install procoder where none was found.
- Do not report success on the version bump while staying quiet about a baseline
  format change or a doctrine change.
