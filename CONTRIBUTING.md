# Contributing to procoder

Thanks for considering a contribution. procoder is a quality gate that holds
other repositories to a standard, so it holds itself to the same one — the
bar here is deliberately boring: small diffs, root causes, and a runnable
check for every piece of non-trivial logic.

## Before you write code

Open an issue first for anything beyond a small fix — a new check, a new
platform, a behavior change. The issue templates ask for what a reviewer
needs; a short conversation there saves a large diff being reworked later.

Bug fixes go to the root cause. Before editing, find every caller of what
you are about to touch — one guard where all paths converge beats a patch in
the one path the issue named.

## Setting up

The CLI is a single Go module (Go 1.23, see `go.mod`), no build system
beyond the toolchain:

```sh
go build ./cmd/procoder     # the binary
go test ./...               # the suite
```

Some tests exercise real tools (gitleaks, eslint, shellcheck, ...) and skip
themselves when the tool is not installed. `go run ./cmd/procoder doctor`
tells you which tools this repository wants and how to install them.

## The gate applies here too

procoder governs its own repository. Before opening a PR:

```sh
go run ./cmd/procoder check    # the full commit gate over your changed files
go run ./cmd/procoder git      # the pre-finish status: branch, hygiene, templates
go test ./...
```

CI runs the same gate plus the test matrix on macOS, Ubuntu, and Windows —
anything red there blocks the merge.

A few repo-specific rules:

- **Don't rebuild `dist/`.** The per-platform binaries under `dist/` are
  stamped with the release version and rebuilt by the maintainer at release
  time; a PR that touches them will be asked to drop those changes.
- **Don't bump versions.** The version lives in several manifests at once
  and moves only in release commits. The pinned tool versions in
  `.github/tool-versions.env` are the same rule with a different owner: a
  weekly workflow opens one PR per tool that is behind, so a hand-written
  bump collides with it. Report a tool that needs upgrading sooner rather
  than editing that file.
- **Changelog entries ride releases**, not PRs — describe the change well in
  the PR body and it will be told in `CHANGELOG.md` when it ships.
- **Every non-trivial change leaves a test behind** — the smallest thing
  that fails if the logic breaks.

## Pull requests

Fill in the PR template — What, Why, How, Testing. "Tests pass" needs the
command that proved it. Keep commits in the repo's voice: a lowercase,
present-tense subject that says what the change does.

Documentation for user-visible behavior lives in `docs/` (the mkdocs site)
and in the command's usage text; a change that alters what a user sees
updates both, and the docs gate will tell you if a reference went stale.

## Reporting security issues

Not in a public issue — see [SECURITY.md](SECURITY.md).

## Conduct

Participation in this project is covered by the
[Code of Conduct](CODE_OF_CONDUCT.md).
