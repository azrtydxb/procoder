# The fixture repository, built from a script rather than copied

Status: open
Created: 2026-08-24
Epic: e2e-campaign
Sprint: -

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

- [ ] A script builds a fixture repository from `git init` alone,
      carrying compilable source in all twelve claimed languages plus a
      test suite, CI workflow, docs and manifests, and rebuilding it
      twice produces identical trees.
- [ ] Each of the twelve languages is present with at least one file per
      extension group procoder's tool table names, so no `reg(...)` row
      goes unexercised.
- [ ] The script is idempotent: run against an existing fixture it
      rebuilds from scratch rather than layering onto the old one.

## Evidence
