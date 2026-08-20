# Update AGENTS.md & skill with upgrade instruction

Status: done 2026-08-20
Created: 2026-08-20
Epic: self-update
Sprint: -

## Description

Add documentation to AGENTS.md and SKILL.md about the upgrade workflow. AI coders should notice when procoder reports a newer version and ask the user to upgrade.

## Acceptance criteria

- [x] `AGENTS.md` gains a new "Upgrading procoder" subsection (or integrates into existing sections)
- [x] AGENTS.md instructs: if procoder prints a version warning, ask the user if they want to upgrade
- [x] AGENTS.md documents `procoder self-upgrade` as the upgrade command
- [x] `skills/procoder/SKILL.md` references the self-upgrade workflow
- [x] Changes are minimal — no rewrite of existing content
- [x] Cross-reference with the principles "Asking the user" section (from the ask feature) if applicable

## Evidence

- AGENTS.md and skills/procoder/SKILL.md document both commands and instruct the agent to report a newer version and ask the user rather than upgrading on their behalf.
- All eleven derived host rule files regenerated; `procoder agents` reports every one matching AGENTS.md.
- docs/commands.md carries the full section: what the check does, what an unanswered check means, the config knob, the consent rule, the backwards refusal, and the package-manager behaviour with its heuristic stated as one.
- Usage text lists both commands; docs.Commands includes self-upgrade, pinned by TestUsageAndCoverageListAgree.
- `procoder docs` 0 blocking; `procoder check` 0 blocking.
