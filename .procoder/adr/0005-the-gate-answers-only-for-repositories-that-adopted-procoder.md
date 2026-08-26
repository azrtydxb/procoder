# 0005 — the gate answers only for repositories that adopted procoder

Status: accepted
Date: 2026-08-26

## Context

A contributor cloned an upstream project they do not own, changed two
files, and was handed nineteen findings. Seventeen were procoder's own
conventions applied to a project that had never heard of it: its
`AGENTS.md` was "drifted" from templates procoder never wrote it from, its
missing `.procoder/github/PULL_REQUEST_TEMPLATE.md` was a finding, its
formatter is Biome and procoder disagreed with it. The eighteenth was a
constant whose name ends in `_STORE_KEY`, on line 4,423 of a file whose
change sat 2,500 lines away — not a credential, not written by that
commit. The project's own gate was green throughout.

The only way through was `--no-verify`, and that is the actual damage. Not
the noise: the noise is merely irritating. The damage is that the escape
hatch is all-or-nothing, so a gate that is wrong seventeen times out of
nineteen teaches its user to switch off the two that were right. A check
nobody can afford to leave on is not protecting anything.

Two facts made this inevitable rather than a bug. procoder had exactly one
mode, and it assumed the repository in front of it had asked for procoder's
opinions. And every content check read whole files, which is correct for
code you wrote and wrong for code you did not.

## Decision

The gate has two scopes, decided from the repository and never from the
machine.

**Adopted** — a `.procoder/` directory, or an `AGENTS.md` that names
procoder. Everything runs, exactly as before. This is the only mode that
existed, and it must remain byte-identical: an adopting repository loses
nothing.

**Universal** — neither of those. Only the checks that are true regardless
of house style run: a credential, an oversized blob, a conflict marker, a
junk file, and an AI-attribution trailer nobody wrote. No repository wants
any of those, whatever its conventions.

And in the universal scope, the checks that read file **content** — secrets
and conflict markers — see only the lines this commit added or changed.
Checks about a file's **existence** — oversized, junk — do not narrow,
because a file this commit introduces is this commit's, all of it.

Both callers go through one function, `gate.ScopeFor`, so `procoder check`
and the pre-commit hook cannot give the same repository different answers.

## Consequences

**Absence of evidence is not adoption.** A repository showing no sign of
procoder is treated as not having adopted it. The failure direction is
deliberately toward saying less about somebody else's code.

**The reduced gate says it is reduced.** Every run prints the mode and why,
and the universal summary says how many files went unchecked rather than
reporting "0 clean" — a quieter gate that does not announce itself is
indistinguishable from a clean one, which is the failure this project
exists to prevent.

**Formatting is a house rule.** This is the least obvious consequence and
the one most likely to be argued. gofmt looks universal, but a repository
may run gofumpt, or Biome, or nothing, and rewriting somebody's files to
procoder's taste is precisely the overreach reported. It does not run in
the universal scope.

**Either mode can be forced** — `[gate] scope` in `.procoder/config.toml`,
or `PROCODER_GATE_SCOPE` for a fork that cannot carry config without that
itself being a change the contributor does not want to make. Config beats
the environment: the file is the repository's deliberate choice, the
variable is whoever's shell this happens to be.

**A pre-existing secret in an adopting repository still blocks.** The
argument cuts both ways there — it is not your commit's fault, and it is
still a credential in your repository, and you asked to be told. Narrowing
in that case stays an open question; this decision does not touch it.

**Adoption is cheap to claim and cheap to fake.** Anybody can add an empty
`.procoder/`. That is fine: adoption is a statement of intent, not a
security boundary, and the cost of a false claim falls on whoever made it.
