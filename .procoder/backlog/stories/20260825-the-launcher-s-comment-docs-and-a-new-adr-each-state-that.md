# The contract that changed is written down

Status: done 2026-08-25
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 020-the-binaries-leave-the-tree-and-ci-builds-them

## Description

`hooks/launcher.sh` says today: "marketplace install, no runtime, no
network." That was a real property and it is being spent.

The launcher's own comment, the documentation, and a new ADR each say what
is true afterwards: the first run fetches over the network, what happens
when it cannot, and why the trade was made. An ADR because a changed mind
supersedes rather than rewrites, and somebody will ask why procoder
reaches the network at session start.

## Acceptance criteria

- [x] The launcher's comment, `docs/`, and a new ADR each state that the
      first run fetches over the network, and what happens when it
      cannot.
- [x] The ADR records what was given up — offline first-run, and
      third-party verifiability — not only what was gained.

## Evidence

- `hooks/launcher.sh` opens by saying the binary is not committed, that
  the first run needs the network, and that this replaces a contract which
  read "marketplace install, no runtime, no network" — pointing at ADR
  0004 for why and stating the failure rules underneath it.
- **ADR 0004** records the decision, the three rejected alternatives (a
  committed Go bootstrap, CI committing `dist/` back, a plugin install
  hook), and — in its own section — what the decision costs: the offline
  first run, third-party verifiability, a release with no assets breaking
  installs rather than upgrades, and the window between a version bump
  landing and CI publishing.
- **The docs were wrong, not merely out of date.**
  `docs/how-to-install-manually.md` told a reader to clone and put
  `dist/darwin-arm64` on PATH — a directory that is now empty. It
  downloads from the release and verifies against `SHA256SUMS` before
  running anything, which is what the launcher does for plugin users.
  `docs/architecture.md` claimed "committed with the plugin: no npm, no
  network at hook time, air-gapped installs included"; it now says what is
  true and points at the ADR. `docs/commands.md` and `docs/portability.md`
  are corrected for the same reason.
