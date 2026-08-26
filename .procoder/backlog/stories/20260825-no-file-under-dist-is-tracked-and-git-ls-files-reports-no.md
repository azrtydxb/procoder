# The binaries leave the tree

Status: done 2026-08-25
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

- [x] No file under `dist/` is tracked, and `git ls-files` reports no
      binary anywhere in the tree.
- [x] The assertion is a test rather than a note, so a future addition
      fails rather than being noticed later.

## Evidence

- `git rm -r --cached dist` and `/dist/` in `.gitignore`, with a comment
  saying where the binaries come from now.
- `TestNoExecutableIsCommitted` in `internal/audit` reads every tracked
  file's first bytes for ELF, Mach-O (both endiannesses, 32 and 64, and
  universal) and PE magic. A test rather than a note, because the next
  convenient exception puts 39MB back a release at a time and a note
  cannot fail.
- **Executables, not "anything binary", and that is deliberate.** The
  repository legitimately tracks PNG logos under `brand/` and
  `docs/assets/`; a test that failed on those would be deleted rather than
  obeyed, and would have taught nobody anything.
- proved by: `git add -f dist/darwin-arm64/procoder` — the test names it
  and says how to undo it. That mutation was run.
