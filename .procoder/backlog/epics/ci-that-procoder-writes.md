# ci-that-procoder-writes

Status: open
Created: 2026-08-22
Spec: ci-that-procoder-writes @ c4f650f98475

## Description

Procoder's second tier becomes portable. Today the whole-tree checks
exist as a workflow file in this repository, so every other repository
that adopts Procoder gets a commit gate and nothing behind it — and the
domain that polices CI cannot see the gap, because `ciops.Check` reports
a repository with no workflows as clean.

This epic makes the tier something Procoder can describe and check
anywhere: emitted per repository from what that repository actually
contains, configured by its own config, overridable block by block, and
reported against when an existing workflow omits it. One unit of value,
because a generator nobody is told they need and a check with nothing to
suggest are each half a feature.

Seeded from .procoder/specs/ci-that-procoder-writes.md — one story per acceptance criterion.
