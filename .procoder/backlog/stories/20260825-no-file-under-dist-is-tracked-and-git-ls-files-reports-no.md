# The binaries leave the tree

Status: open
Created: 2026-08-25
Epic: ci-built-binaries
Sprint: 020-the-binaries-leave-the-tree-and-ci-builds-them

## Description

`dist/` stops being tracked. Nothing binary is committed in its place
— not a stub, not a compressed archive, not a fallback for one platform.

This is the point of the epic and it is worth an assertion of its own: a
test that reads the tracked file list and fails on anything that is not
text. Otherwise the next convenient exception puts 39MB back a release at
a time.

## Acceptance criteria

- [ ] No file under `dist/` is tracked, and `git ls-files` reports no
      binary anywhere in the tree.
- [ ] The assertion is a test rather than a note, so a future addition
      fails rather than being noticed later.

## Evidence
