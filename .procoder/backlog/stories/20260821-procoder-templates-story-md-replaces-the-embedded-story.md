# `.procoder/templates/story.md` replaces the embedded story template in `backlog story` output, and a repository without the file gets the embedded one unchanged.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

Nine templates drove the quality chain as embedded constants with no way in, so a team with a house format had to choose between the domain and their own shape.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] `.procoder/templates/story.md` replaces the embedded story template in `backlog story` output, and a repository without the file gets the embedded one unchanged.

## Evidence

`go test ./internal/templates/ -run TestTheRepositorysTemplateWinsAndAbsentMeansDefault`: PASS. Proved by mutation: returning the embedded body even when a file is present ignores the team's format silently. Run end to end — `.procoder/templates/story.md` containing HOUSE STYLE produced a story in that shape; removing it restored Procoder's.
