# On a fixture whose `db/migrate` gained two files since the baseline, the output reads "2 migration(s) added since your last sync" and lists both names.

Status: done 2026-08-19
Created: 2026-08-19
Epic: sync-awareness
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Migrations added are counted and named.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] On a fixture whose `db/migrate` gained two files since the baseline, the output reads "2 migration(s) added since your last sync" and lists both names.

## Evidence

- TestMigrationsAddedAreCounted; TestMigrationsRemovedReadAsChangedNeverNegative keeps a squash from reading as a negative count.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
