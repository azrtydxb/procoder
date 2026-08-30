# store: the guard tests — coverage, no direct IO, no deps, golden parity

Status: closed 2026-08-28
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

- [x] `TestStoreCoversEveryPathConstant` — every `.procoder/` string literal in `internal/` and `cmd/` outside `internal/store` and outside `_test.go` files appears in the store's declared path list; a new constant without a pair fails it.
- [x] `TestNoDirectProcoderFileIO` — no package outside `internal/store` calls a stdlib read/write/create/remove against a `.procoder/` path; reintroducing one fails it.
- [x] `TestNoModuleDependencies` — go.mod contains no `require`.
- [x] `TestMigrationOutputUnchanged` — `procoder status`, `procoder check`, `procoder config` and the four hook entrypoints produce stdout byte-identical to committed goldens, over a fixture repository with git dates fixed to 2026-01-01T00:00:00Z.
- [x] The goldens were generated at the commit preceding task 1, and the evidence records which commit that was.
- [x] `procoder check` is clean.

## Evidence

- `internal/store/coverage_test.go` and `internal/store/deps_test.go`,
  committed as 9720427. No production code — this task is the evidence for
  the other six.
- TestNoDirectProcoderFileIO walks internal/ and cmd/ with go/ast. It found
  the last holdout, status reading the index meta.json, and one false
  positive worth naming: the self-upgrade's `.procoder-upgrade-*` temp
  directory is a NAME, not a path. The prefix test now matches
  `.procoder/` or exactly `.procoder`, never `.procoder-`.
- os.Stat is deliberately not guarded: asking whether something exists is
  not reaching past the store, and gate adoption legitimately stats
  .procoder/ to decide whether a repository opted in.
- TestStoreCoversEveryPathConstant holds a hand-edited list of 41 paths,
  each naming the operation that serves it; four say plainly that they are
  prose or a gitignore prefix rather than a path. A new .procoder/ path
  cannot be added without deciding which operation serves it.
- TestNoModuleDependencies reads go.mod. The lock is an O_EXCL file rather
  than flock because a portable flock/LockFileEx pair costs
  golang.org/x/sys — reasoning only worth anything while the module really
  has no dependencies, so it gets a test rather than a comment.
- Mutation-checked, snapshot taken and restored around each: adding a
  direct os.ReadFile on .procoder/docs/RULES.md fails the IO guard;
  deleting one knownPaths entry fails the coverage guard with
  `path ".procoder/todo" has no store pair`; appending a require block to
  go.mod fails the dependency guard.
- One earlier mutation attempt was NOT valid and is recorded as such:
  reverting the status.go call to os.ReadFile broke the build on unused
  imports rather than failing the test, so it proved nothing. The valid
  mutation adds a call to a file that already imports os and filepath.
