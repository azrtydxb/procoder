# File-level checks do not narrow, and that is deliberate

Status: done
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: 021-procoder-tells-third-party-repositories-only-what-is-true

## Description

The line between the two kinds. Secrets and conflict markers read a
file's contents, so in somebody else's repository they narrow to the diff.
Size and junk are about a file existing at all — and a file this commit
adds is this commit's, every byte of it.

A 12MB blob does not become less yours because you only wrote part of it.

## Acceptance criteria

- [x] An oversized file and a junk file that the commit introduces still
      block in a non-adopting repository.

## Evidence

`TestAJunkFileStillBlocksInANonAdoptingRepository` and
`TestAnOversizedFileStillBlocksInANonAdoptingRepository`, each killed by
dropping its check from `CollectUniversal`.

Worth recording: narrowing these was assumed to be a risk and is not.
`JunkFiles` and `Oversized` findings carry no line number, and
`NarrowToDiff` keeps line-less findings by construction — verified, not
assumed, by `TestNarrowToDiffKeepsWholeFileFindings`. Routing them through
the narrower is a no-op, so the criterion as written could not fail. See
issue #186.
