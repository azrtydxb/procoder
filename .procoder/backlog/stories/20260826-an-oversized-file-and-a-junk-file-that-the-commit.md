# File-level checks do not narrow, and that is deliberate

Status: open
Created: 2026-08-26
Epic: adoption-aware-gate
Sprint: -

## Description

The line between the two kinds. Secrets and conflict markers read a
file's contents, so in somebody else's repository they narrow to the diff.
Size and junk are about a file existing at all — and a file this commit
adds is this commit's, every byte of it.

A 12MB blob does not become less yours because you only wrote part of it.

## Acceptance criteria

- [ ] An oversized file and a junk file that the commit introduces still
      block in a non-adopting repository.

## Evidence
