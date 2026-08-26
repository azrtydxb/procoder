# adoption-aware-gate

Status: open
Created: 2026-08-26
Spec: adoption-aware-gate @ fbe56e4ede6a

## Description

Clone a third-party repository, change two files, commit — and procoder
blocks it with nineteen findings, seventeen of which are about procoder
rather than about the change (#172). The repository's own gate was green:
tsc clean, Biome clean, 29 of 29 tests passing.

Twelve of them demanded procoder's agent rule files in somebody else's
repository, and called that project's own `AGENTS.md` "drifted". Two
demanded its README carry a version procoder does not set. Two called files
unformatted that the repository's declared formatter calls clean. One asked
for ESLint in a Biome project. And one flagged a constant named
`..._STORE_KEY` on line 4,423 of a file whose diff sits 2,500 lines away.

The gate is all-or-nothing, so the way through was `--no-verify` — which
also switches off the checks that were worth having. That is the real
damage: not the noise, but that the noise trains people to disable the
gate, and then it protects nothing.

Two rules fix it. A repository that never adopted procoder gets only the
checks that are true anywhere — no secret, no 12MB blob, no conflict
marker, no junk file, no trailer nobody wrote — and nothing about house
style. And in that repository the content checks read only the lines this
commit wrote, because in somebody else's code the diff is the only part
that is mine to answer for.

An adopting repository loses nothing at all. That is the constraint the
whole epic is built around, and the one a test has to hold.
