# store: the guard tests — coverage, no direct IO, no deps, golden parity

Status: open
Created: 2026-08-28

## Description

Plan task 7 of .procoder/plans/service-state-seam.md.

This task adds no production code. It is the evidence that the previous
six did what they claimed, and the thing that stops the seam eroding
after the change lands.

The golden files matter in one specific way: they must be generated from
the commit BEFORE task 1, in a worktree checked out at that commit.
Goldens taken after the migration would prove nothing at all.

## Acceptance criteria

- [ ] `TestStoreCoversEveryPathConstant` — every `.procoder/` string literal in `internal/` and `cmd/` outside `internal/store` and outside `_test.go` files appears in the store's declared path list; a new constant without a pair fails it.
- [ ] `TestNoDirectProcoderFileIO` — no package outside `internal/store` calls a stdlib read/write/create/remove against a `.procoder/` path; reintroducing one fails it.
- [ ] `TestNoModuleDependencies` — go.mod contains no `require`.
- [ ] `TestMigrationOutputUnchanged` — `procoder status`, `procoder check`, `procoder config` and the four hook entrypoints produce stdout byte-identical to committed goldens, over a fixture repository with git dates fixed to 2026-01-01T00:00:00Z.
- [ ] The goldens were generated at the commit preceding task 1, and the evidence records which commit that was.
- [ ] `procoder check` is clean.

## Evidence

