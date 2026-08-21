# A `## checks` list in a domain's RULES.md replaces that domain's baseline check set; a RULES.md with no such section keeps the default.

Status: done 2026-08-21
Created: 2026-08-21
Epic: configurable-defaults
Sprint: 010-configurable-defaults-the-repository-decides-and-says-so

## Description

The RULES.md pattern existed in three of about forty packages. Done means the lint domain has it too, and a list replaces rather than appends.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] A `## checks` list in a domain's RULES.md replaces that domain's baseline check set; a RULES.md with no such section keeps the default.

## Evidence

`go test ./internal/lint/ -run TestACheckListReplacesTheBaseline`: PASS, including that a rules file with prose but no checks section keeps the default — a missing section must never read as "no checks at all". Proved by mutation: joining the list onto the baseline makes a repository that narrowed its checks still receive everything it excluded. Run end to end — the baseline reported a garbage-value analyser finding; `## checks` with `readability-*` reported an identifier-length finding instead.
