# A `docs: none — internal refactor` line in the commit message clears the obligation; an empty reason does not.

Status: done 2026-08-19
Created: 2026-08-19
Epic: docs-gate
Sprint: 003-enforcement-the-commit-gate-and-the-docs-obligation-0300

## Description

The ack clears it, an empty reason does not.

<!-- The user story: who needs what, and why. What "done" looks like in
     the reader's terms — a title is not a description. -->

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A `docs: none — internal refactor` line in the commit message clears the obligation; an empty reason does not.

## Evidence

- Live: `git commit -m msg -m "docs: none — internal helper only"` was allowed; the same with an empty reason was denied. TestAcknowledgmentClearsOnlyWithAReason green. The message reaches the gate via gate.RunWith / gitcmd.CollectFor, threaded from the hook's -m parsing.

<!-- Filled at close time: the commands run and what their output proved,
     one line per criterion. Empty evidence keeps the story open. -->
