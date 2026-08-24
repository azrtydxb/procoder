# `[planning] method = "bmad"` with a fixture BMad install reports sprint state from `sprint-status.yaml`, with each story's status, and does not report from `.procoder/backlog/`.

Status: done 2026-08-24
Created: 2026-08-24
Epic: planning-methodology
Sprint: 013-the-analysis-phase-and-the-seam-that-lets-bmad-plan

## Description

The whole promise of track 2. A repository that plans in BMad should see
its own sprint reflected back by procoder, not an empty procoder backlog
next to a BMad one that is actually being worked.

Done means sprint state comes from `sprint-status.yaml` with each story's
status, and `.procoder/backlog/` is not the source.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `[planning] method = "bmad"` with a fixture BMad install reports sprint state from `sprint-status.yaml`, with each story's status, and does not report from `.procoder/backlog/`.

## Evidence

- `go test ./internal/planning/ -run TestSprintStateComesFromTheArtifactsOnDisk` — `1 done, 2 open` read from a fixture `sprint-status.yaml`, each outstanding entry named, `optional` counted as neither. Verified end to end: `procoder status` printed the sprint from the artifacts, not from `.procoder/backlog/`. Also covers `TestTheOutputFolderIsReadFromTheInstallation` — a repository that configured a non-default `output_folder` is read rather than assumed.
