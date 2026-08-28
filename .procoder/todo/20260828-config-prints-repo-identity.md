# config: procoder config prints the repo identity and its rung

Status: open
Created: 2026-08-28

## Description

Plan task 4 of .procoder/plans/service-state-seam.md.

The identity computed in task 3 has no consumer until the daemon exists,
which would make it untestable new work sitting on phase 1's critical
path. Printing it in `procoder config` makes it observable and testable
now, with no daemon.

It goes in Report and deliberately NOT in cfg.Settings: identity is not a
setting with a default that can be relaxed, and putting it in that table
would make it look like one.

## Acceptance criteria

- [ ] `TestConfigPrintsIdentityRung` — one case per rung, each asserting a line beginning `repo identity` carrying both the key and the rung wording: `[service] repo in .procoder/config.toml`, `origin remote`, `first remote alphabetically: <name>`, `no remote — repository root path`.
- [ ] The identity line does not appear in the settings table, and `cfg.Settings` gains no entry for it.
- [ ] `procoder check` is clean.

## Evidence

