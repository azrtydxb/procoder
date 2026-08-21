# configurable-defaults

Status: done 2026-08-21
Created: 2026-08-21
Spec: configurable-defaults @ 4c9d872b9da0

## Description

<!-- What this epic delivers and why it is one coherent unit of value.
     Stories reference this epic by its file name. -->

Seeded from .procoder/specs/configurable-defaults.md — one story per acceptance criterion.

## Spec change during the sprint

One acceptance criterion changed while the sprint was running, which is
why the fingerprint above is not the one this epic was seeded with. D-1's
example — `[tools] php = "pint"` — turned out to be impossible: pint
writes in place, as do phpcbf and php-cs-fixer, so offering any of them
would break the print-don't-write contract the restriction exists to
protect. D-6 records the narrower rule and the criterion now names biome.

The story seeded from the original criterion is closed as superseded
rather than deleted: it is where the evidence lives, and a criterion
proven impossible is a result, not an absence.
