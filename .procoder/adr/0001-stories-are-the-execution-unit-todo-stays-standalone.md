# 0001 — Stories are the execution unit; todo stays standalone

Status: accepted
Date: 2026-08-19

## Context

Building the backlog system raised the question of whether the existing
quality-gated todo tasks should become the leaf under stories, be
absorbed by them, or stay separate. Todo predates the backlog and is
used for ad-hoc work with no spec behind it.

## Decision

The user story is the execution unit of spec-based work and carries the
full close rigor itself. The todo list stays untouched as the
standalone track for work not born from a spec. Decided by Pascal in
the backlog spec interview, 2026-08-19.

## Consequences

Easier: no migration, two clear lanes (spec-born vs ad-hoc), story
close and todo close share one discipline without sharing state.
Harder: two places to look for open work — the board and the todo list
— which `backlog board` and `todo list` each report separately.
