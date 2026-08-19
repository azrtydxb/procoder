# The JS path appends the pattern after `--` and the output states that filtering is delegated to the runner, pinned by a unit test over the constructed argv.

Status: done 2026-08-19
Created: 2026-08-19
Epic: inner-loop
Sprint: 004-the-loop-single-test-run-status-handoff-env-and-ci

## Description

Delegated filtering is said, not implied.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] The JS path appends the pattern after `--` and the output states that filtering is delegated to the runner, pinned by a unit test over the constructed argv.

## Evidence

- TestCargoJSGradleMavenFilterArgs asserts the JS argv is `<mgr> test -- <pattern>` and the Detail says filtering is delegated to the runner.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
