# 0003 — what 1.0 promises

Status: accepted
Date: 2026-08-20

## Context

Procoder reached 0.32.11 as a working product: ten domains, the quality
chain, the universal agent layer, a documentation site, and 25 recorded
lessons whose adaptations all landed. People are being asked to install
it and let it gate their commits.

Before 1.0 every version was implicitly provisional — a command could be
renamed, an output line reworded, a file format changed, and the only
cost was a paragraph in the changelog. Several such changes shipped, and
that freedom was worth having.

It stops being worth having the moment someone scripts against the
output or commits `.procoder/` files to a repository that other people
work in. A version number that says 1.0 and behaves like 0.x is worse
than no version number at all, so what it covers has to be written down
before it is claimed rather than argued about afterwards.

## Decision

Release 1.0.0, and treat these as the public interface — a breaking
change to any of them requires a major version:

- **Command names and their subcommands.** `procoder check`,
  `procoder backlog close story`, and the rest keep their spelling.
- **Exit codes.** 0 clean, 1 findings or refusal, 2 usage. A command
  that exits 0 today does not start exiting 1 for the same input.
- **Blocking semantics.** What blocks the gate stays blocking; what is
  information stays information. Adding a new blocking check is a major
  change, because it can fail a build that passed yesterday. Adding an
  informational finding is not.
- **The `.procoder/` file formats** that a repository commits: config
  keys, the spec/plan/todo/backlog/adr document shapes, the epic `Spec:`
  fingerprint, the rules files' machine-read sections.
- **The hook contract**: the payload shapes read on stdin and the
  envelopes written on stdout, per host.

Explicitly NOT covered, because pinning them would freeze the product:

- **The wording of report lines.** Verdicts are for people to read.
  Anything scripting against report prose is relying on something never
  promised; exit codes are the contract.
- **The default rules content** — PRINCIPLES, the review rubric, the
  documentation rules. These are defaults a repository overrides
  (D-OVERRIDE); improving them is the point.
- **`internal/` packages.** The Go API is not published.
- **The derived index format** under `.procoder/index/`, which is
  gitignored and rebuilt.

Two changes land in 1.0.0 rather than after it, because both would be
breaking later: nothing else. The SHA-1 spec fingerprint stays as it is
and is now covered by the promise above — it was reviewed for this
release and kept deliberately, since changing the digest would flag
drift on every epic already seeded.

Alternatives considered:

- **Stay on 0.x indefinitely.** Rejected: the product is being
  installed and used, and 0.x is a signal that it should not be trusted
  yet. That signal is now false.
- **Promise nothing beyond command names.** Rejected: the files a
  repository commits are the part that hurts most when it changes, and
  they are exactly what a user cannot work around.
- **Promise the report wording too.** Rejected: it would freeze every
  message against improvement, for the benefit of a scripting pattern
  the exit codes already serve better.

## Consequences

Easier: a user can pin `1.x` and expect their `.procoder/` directory,
their scripts' exit-code handling, and their gate's verdicts to keep
meaning what they mean. Contributors get a clear question for any
change — does this alter something in the covered list?

Harder: new blocking checks now need a major version, which means good
ideas will wait or ship as informational first. That is the intended
cost. The alternative is a gate whose behaviour changes under people who
did not ask for it, which is the failure this product exists to argue
against.

This record is the reference when the question comes up. A change of
mind supersedes it rather than editing it.
