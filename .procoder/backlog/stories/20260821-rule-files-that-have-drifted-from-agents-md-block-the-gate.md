# Rule files that have drifted from AGENTS.md block the gate, making the sentence already in docs/commands.md true.

Status: done 2026-08-21
Created: 2026-08-21
Epic: checks-that-run-themselves
Sprint: 011-checks-that-run-themselves-the-gate-for-the-change-ci-for

## Description

`docs/commands.md` already says the gate blocks on agent-rule drift. It
did not. A stale rule file is another agent being told something this
repository stopped believing, and the documentation promising otherwise
is worse than silence.

Done means the sentence is true: drifted host files block, a repository
with no AGENTS.md is asked nothing, and a master that cannot be read
blocks rather than passing as absent.

## Acceptance criteria

<!-- Each criterion is testable. Check a box ONLY when it is verifiably
     true — the closer will ask for the evidence. -->

- [x] Rule files that have drifted from AGENTS.md block the gate, making the sentence already in docs/commands.md true.

## Evidence

- `go test ./internal/portability/ -run TestDriftedRuleFilesBlock` — a drifted host file produces a blocking finding; TestMatchingFilesAndNoAgentLayerAreSilent covers the no-AGENTS.md case and TestAnUnreadableMasterIsNotAnAbsentAgentLayer the unreadable one. (#130)
