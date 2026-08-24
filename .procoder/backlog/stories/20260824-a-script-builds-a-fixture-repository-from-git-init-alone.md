# The fixture repository, built from a script rather than copied

Status: done 2026-08-24
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 014-a-fixture-that-is-not-this-repository-and-a-clean-pass-over

## Description

Nothing else in this epic can run until there is a repository that is
not this one to run it against. It has to be a real repository, not a
directory of samples: `git init`, a first commit, a branch, a remote
shape procoder's git-aware commands can read.

It carries compilable source in each of the twelve languages procoder
claims to format — Go, Python, Rust, C/C++, shell, Java, Kotlin, Swift,
Ruby, Dart, C#, PHP — because a formatter that is never handed a file in
its language is a formatter nobody has tested. Alongside them: a test
suite in more than one runner, a CI workflow that predates procoder,
docs with internal and external links, and dependency manifests.

Built by script, not committed. The broken pass will plant secrets and
vulnerable manifests in it, and committing those here would trip this
repository's own gate — correctly. A script also means a finding can be
reproduced from `git init` rather than from whatever state somebody
happened to be in.

## Acceptance criteria

- [x] A script builds a fixture repository from `git init` alone,
      carrying compilable source in all twelve claimed languages plus a
      test suite, CI workflow, docs and manifests, and rebuilding it
      twice produces identical trees.
- [x] Each of the twelve languages is present with at least one file per
      extension group procoder's tool table names, so no `reg(...)` row
      goes unexercised.
- [x] The script is idempotent: run against an existing fixture it
      rebuilds from scratch rather than layering onto the old one.

## Evidence

- `scripts/build-e2e-fixture.sh` builds from `git init` and commits;
  `git rev-parse HEAD^{tree}` after two consecutive builds returned
  `245096e3` both times, and the commit hash matched too — identity and
  clock are fixed in the script so a rebuild is byte-identical.
- Every extension in `internal/tools/tools.go`'s `reg(...)` table is
  present: a loop over all thirty-three (`.go .py .pyi .js .jsx .mjs
.cjs .ts .tsx .mts .cts .json .css .scss .md .yaml .yml .html .rs .c
.h .cpp .cc .cxx .hpp .sh .bash .java .kt .kts .swift .rb .rake .dart
.cs .php`) reported none missing. Thirteen rows, not the twelve
  languages the spec's prose named — the prettier web row is the
  thirteenth, and counting from the table rather than from memory is why
  it is covered.
- `rm -rf` then rebuild is the script's first act, so a second run
  replaces rather than layers; proved by the identical-tree result above,
  which would drift if anything survived.
- The fixture's own suites run: `go test ./...` ok, `npm test` 2 passed,
  `pytest` 2 passed. `mvn` is absent from this machine and `procoder
test` reports it NOT run rather than green.
