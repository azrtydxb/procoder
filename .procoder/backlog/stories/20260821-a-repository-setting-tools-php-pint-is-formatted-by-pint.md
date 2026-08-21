# A repository setting `[tools] php = "pint"` is formatted by pint, and `procoder doctor` lists pint rather than prettier. — SUPERSEDED: the criterion was impossible, see D-6

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

Superseded by D-6 and replaced with the biome criterion. Building this
story is what proved the original impossible, so it is closed as a result
rather than deleted as an absence — a reader looking for why pint is not
on the menu should find this file.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The criterion below was proven impossible and the story is closed as
      superseded; the spec records why as D-6, and the work it asked for is
      delivered under the biome criterion.
      ~~A repository setting `[tools] php = "pint"` is formatted by pint,
      and `procoder doctor` lists pint rather than prettier.~~

## Evidence

The criterion could not be met as written. `pint` writes the file in
place; so do `phpcbf` and `php-cs-fixer`. Each was run and checked rather
than assumed — `php-cs-fixer fix -` reports which files CAN be fixed
instead of emitting the fix, and `--diff` emits a diff. Offering any of
them would break the print-don't-write contract that D-1's restriction
exists to protect, so the menu is narrower than D-1 first read.

D-6 in .procoder/specs/configurable-defaults.md records the rule: a tool
reaches the menu by being able to emit formatted source on stdout. biome
does it through `--stdin-file-path`, verified before the code was written
(`biome format --stdin-file-path=messy.js < messy.js` printed clean output
with the file's digest unchanged), and the acceptance criterion now names
it. The work that criterion asked for is delivered under the biome
criterion and its story.
