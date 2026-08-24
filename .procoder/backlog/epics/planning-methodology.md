# planning-methodology

Status: done 2026-08-24
Created: 2026-08-24
Spec: planning-methodology @ 86cabf0d9508

## Description

Procoder learns the half it never built — planning — and lets a
repository opt out of that half entirely in favour of the tool that
already does it well.

Track 1 grows the capability in procoder's own code: a multi-lens
review that applies judgment where the gate only applies tooling, an
analysis phase for reaching a spec worth checking, perspectives as
review stances, and right-sizing that names which entry point a change
belongs at. Track 2 lets a repository set `[planning] method = "bmad"`
and have procoder validate BMad's artifacts instead of demanding its
own, with the governance backbone untouched.

One epic, because the boundary is the point: every capability track 1
adds is one track 2 must not duplicate, and the two only stay coherent
while somebody is holding both. Either track can ship first.

Seeded from .procoder/specs/planning-methodology.md — one story per acceptance criterion.
