# A compound `... && git commit -m x` is detected; `echo "commit"` and `gh pr merge` are not.

Status: done 2026-08-19
Created: 2026-08-19
Epic: gate-enforcement
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The matcher survives real shells.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A compound `... && git commit -m x` is detected; `echo "commit"` and `gh pr merge` are not.

## Evidence

- Live matrix: `go build ./... && git commit -m x` → deny; `git -C . commit` → deny; `echo "git commit"`, `gh pr merge`, `git merge --continue`, `git log --format=%h commit` → silent. TestIsGitCommit covers 22 cases including quoted words and explicit git paths.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
