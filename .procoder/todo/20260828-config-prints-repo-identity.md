# config: procoder config prints the repo identity and its rung

Status: closed 2026-08-28
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

- [x] `TestConfigPrintsIdentityRung` — one case per rung, each asserting a line beginning `repo identity` carrying both the key and the rung wording: `[service] repo in .procoder/config.toml`, `origin remote`, `first remote alphabetically: <name>`, `no remote — repository root path`.
- [x] The identity line does not appear in the settings table, and `cfg.Settings` gains no entry for it.
- [x] `procoder check` is clean.

## Evidence

- `internal/config/report.go` and `internal/config/report_test.go`,
  committed as 8d60453.
- TestConfigPrintsIdentityRung covers all four rungs, asserting the line
  carries both the key and the rung wording. TestIdentityIsNotASetting
  asserts the identity never appears in `cfg.Settings`.
- Mutation-checked: printing only `id.Key` and dropping `id.Source()`
  fails TestConfigPrintsIdentityRung with `line "repo identity
acme/widgets" does not name the rung`.
- docs/configuration.md gained the `[service]` section — the ladder table,
  the URL normalisation, why origin beats an alphabetically earlier
  remote, and when to set the key by hand. This is the first task in the
  seam that owed a real doc rather than a `docs: none` line, because it is
  the first with a user-facing surface.
- `procoder check` clean; the commit gate passed.
