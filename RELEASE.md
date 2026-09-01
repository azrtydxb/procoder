# Releasing

The whole process, for a person or an agent, in order. Every step has a
refusal built in — a step you can skip is a step that has already failed
somewhere. When a step refuses, it is telling you which earlier step is
wrong; go back instead of around.

## 1. Decide the version

Semver over the last tag (`git tag --list 'v*' --sort=-version:refname`).
A contract change — anything that changes what an adopter's agent is
governed by, or what a command prints — is at least a minor bump.

## 2. Sync the version files

Nine files carry the version, and the controller verifies them together;
bump all of them to the new version:

```
plugin.yaml  package.json  gemini-extension.json  README.md
.claude-plugin/plugin.json  docs/index.md
.github/plugin/marketplace.json  .codex-plugin/plugin.json
.github/plugin/plugin.json
```

The list is `[release] files` in `.procoder/config.toml`, not this
paragraph — the file is the answer, this is the pointer.

## 3. Write the changelog entry

`CHANGELOG.md`, newest first, `## <version> — <date>`. The layout and its
rules are the template comment at the top of the file; the non-obvious
ones, that have each blocked a release:

- The entry is the release notes: CI extracts it verbatim and refuses an
  empty one, so what is written here is what a downloader reads.
- Every headline links the PR or issue that shipped it — the link rule
  only accepts pull/issue links. A change that went in as a direct push
  with no PR has a gap: **file the issue it describes, and link that.**
  A retroactive PR is not a substitute; it is a lie about history.
- Every outside contributor cited in the entry is credited in the
  paragraph that cites their work, as a markdown profile link. A bot is
  credited with its suffix, `[@github-actions[bot]](https://github.com/apps/
github-actions)`; the two spellings GitHub uses for one bot
  (`name[bot]` and `app/name`) compare equal and both are accepted.

## 4. Bump the contract if behaviour moved

If `skills/procoder/SKILL.md`'s body — that is, `AGENTS.md` — changed in
a way that governs an adopter differently, the skill's `contract:`
version is lying. Bump `ContractVersion` in
`internal/portability/portability.go`, then regenerate the skill with
`procoder agents` (it prints the refreshed frontmatter; you write it).
The gate blocks while the skill and the constant disagree, so there is no
way to ship a silent contract change.

## 5. The controller

```
procoder release <version>
```

It checks the version files, the changelog entry, the tree, the gate and
the suite, and it ends with `git tag -a <version> -m "<version>"` —
printed, never run. It says what is missing; it does not fix it.

**Run the repository's binary for this, not a hand-built one in a temp
directory.** A stale local build can check the world it was built from
and miss what is new. If no binary is installed, the launcher in
`hooks/` fetches and checksums one; that is the path.

## 6. Commit through a pull request

`main` is pull-request-protected, and the release commit goes to
`main`. Direct pushes are refused by the protection, not by the
controller — the controller cannot see the branch rules, so this is the
step nobody automates. The four required checks (gate, test on the three
OSes) must be green on the branch's head; if the branch goes stale, merge
`main` into it and wait — never `--admin` over a red or a stale state.

## 7. Tag the commit that is on main

After the merge, on the merged commit:

```
git tag -a v<version> -m "<version>"
git push origin v<version>
```

A tag points at a commit. A commit that is not on `main`'s history is
unreachable, and a release is a pointer at its contents: tag early and
the release is a pointer at a place the history will never visit. If it
happens anyway, `git tag -d` and the remote delete, then tag the merged
commit — a re-pointed tag is not a corrected release, it is a new one,
and the tag push re-runs the release job.

## 8. CI publishes; nobody builds

The `release` job in `.github/workflows/ci.yml` runs on the tag push: it
takes the changelog entry as the release notes, builds the five platform
binaries **from the tagged commit** via `scripts/build-dist.sh` (the
version is stamped into the binary through `-ldflags`), and publishes
them with `SHA256SUMS` under the GitHub release.

Do not build release binaries on a machine and do not commit `dist/`.
Binaries were committed once, at a cost of 39MB of history per release —
and shipped the previous version's binaries exactly once, every manifest
green, the gate green, the plugin installing something that reported one
version and behaved like another. The fix was to stop building by hand,
and a local build is how it comes back.

`scripts/build-dist.sh` exists for one purpose: the checksum manifest a
local `dist/` must agree with when the launcher fetches. CI runs it, not
you, for anything that ships.

## 9. Running the newest

- `procoder version --check` — what is newer than the running binary.
  When a session start reports a newer version, say so and ask; it is not
  the binary's decision to install itself.
- `procoder self-upgrade` — after an explicit yes; it verifies the
  download against the published `SHA256SUMS` before installing, refuses
  to move backwards, and steps aside from a package manager's install.
- Host installs (pi and the plugin hosts) point at a released tag, not
  at a working tree, once the release exists. A working-tree install is a
  development state; the todo that tracks each one says when it becomes
  a tag.

## When a step refuses

| Refusal                                               | The earlier step that is wrong                                 |
| ----------------------------------------------------- | -------------------------------------------------------------- |
| `does not contain <version> — bump it`                | 2: a version file missed                                       |
| `no ## <version> heading`                             | 3: the entry is not written                                    |
| a headline "could not be resolved"                    | 3: no issue or PR behind a claim — file it                     |
| "not credited in the paragraph"                       | 3: credit line missing or spelled wrong                        |
| contract note about `SKILL.md`                        | 4: bump `ContractVersion` and regenerate                       |
| `the test suite is not passing`                       | the suite, over what is about to ship — NOT run is never green |
| `main` rejects the push                               | 6: it is a pull request                                        |
| release job "refusing to publish empty release notes" | 3: the entry is whitespace                                     |
| a tag at a commit main does not contain               | 7: delete, re-tag the merged commit, push                      |
