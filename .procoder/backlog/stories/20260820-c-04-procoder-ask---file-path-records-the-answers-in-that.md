# C-04: `procoder ask --file <path>` records the answers in that file against the questions they belong to, and refuses a file it cannot parse rather than recording a partial reading.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

The route an answer takes back in when there was no terminal: the coder relays the questions, writes what the human said into the file, and hands it over. The file is read whole before anything is recorded, because a partial reading of somebody's decisions is worse than refusing the file.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-04: `procoder ask --file <path>` records the answers in that file against the questions they belong to, and refuses a file it cannot parse rather than recording a partial reading.

## Evidence

- TestTheFileRouteRecordsAnswersAndRefusesNonsense records an answer through `ask --file` and reads it back keyed to its question, then hands the same command a file with no answers in it and asserts exit 2 with 'nothing was recorded'.
- The file is parsed whole before anything is recorded: a partial reading of somebody's decisions is worse than refusing the file.
- An answer to a question no domain is currently asking is KEPT rather than dropped, and counted out loud, so a mistyped key does not look like success.
- Live: answered one of this repository's real marketplace-strategy questions through the file route and watched `spec check` go from 4 unanswered to 3.
