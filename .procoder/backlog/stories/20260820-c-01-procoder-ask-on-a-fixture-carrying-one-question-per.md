# C-01: `procoder ask` on a fixture carrying one question per generating domain — an unresolved spec question, an uncleared documentation obligation, a flagged secret, a blocking lint finding — prints all four, each naming its source, and a flagged secret's value appears nowhere in the output or in either file.

Status: done 2026-08-21
Created: 2026-08-20
Epic: interactive-qa
Sprint: 007-interactive-qa-procoder-asks-the-human-instead-of-letting

## Description

Collect the questions every domain already knows it cannot answer — a spec's undecided questions, a documentation obligation nobody cleared, a flagged secret, a lint finding needing judgement — and normalise them into one shape carrying where each came from. A flagged secret's value stays inside the security domain: what a human is asked is whether the flag is real.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] C-01: `procoder ask` on a fixture carrying one question per generating domain — an unresolved spec question, an uncleared documentation obligation, a flagged secret, a blocking lint finding — prints all four, each naming its source, and a flagged secret's value appears nowhere in the output or in either file.

## Evidence

- internal/ask/ask.go collects from all four: spec (spec.OpenQuestions), docs (docs.Obligation), security (security.SecretsChangedFiles) and lint (lint.Files), each question carrying its Source and Origin.
- TestASecretsValueNeverReachesTheQuestion plants an AWS-shaped key in a security finding and asserts it appears in neither the question nor the origin, while the location does — and asserts the other domains DO carry their message, since that is the evidence a human judges.
- Live on this repository: `procoder ask` collected 10 questions across spec, docs and lint, each naming where it came from.
- A domain that cannot answer contributes nothing rather than failing the collection: a linter that is not installed is not a question.
