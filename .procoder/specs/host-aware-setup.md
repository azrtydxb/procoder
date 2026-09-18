# Host-aware setup

Status: complete

## Problem

Setup currently offers every host copy, and adopting one makes the drift gate
require all others. A single-framework project must not acquire unrelated files.

## Users

Plugin users need reliable caller selection; terminal users need an explicit
choice; procoder maintainers need deliberate all-host distribution validation.

## In scope

- [S-1] Select hosts consistently for init and agents, including API requests.
- [S-2] Print additive host declarations and only selected or existing copies.
- [S-3] Validate declared and existing integrations without demanding other hosts.
- [S-4] Carry reliable adapter context and update setup guidance and regressions.

## Out of scope

Installing editor plugins or silently writing generated rules. Changing hook
response envelopes or unrelated pending decisions. Committing or publishing.

## Constraints

Explicit flags override reliable context. Unknown or conflicting context asks
for a choice. No arbitrary path detection. Existing integrations and shared
.procoder state remain intact. Init retains its established gitignore write.

## Interfaces

init and agents accept repeatable --host or exclusive --all. Init also accepts
--yes in either order. Adapters convey PROCODER_HOST in the caller environment.

## Data

.procoder/hosts.json is a printed, user-written array of selected host IDs or
the single all value. Existing rule files remain checked even without a record.

## Edge cases

Unknown IDs, duplicate flags, empty values, mixed all and named hosts, competing
plugin signals, unreadable declarations, missing masters, and legacy Kilo files.

## Failure modes

Invalid setup inputs exit before mutations. Missing or unreadable integration
content is not called clean. No interactive input is consumed over the API.

## Acceptance criteria

- [x] [S-1] `TestSetupSelection` and `TestSetupCLIAPIAndCallerIsolation` cover explicit selection, unknown context, precedence and CLI/API parity.
- [x] [S-2] `TestHostSetupPrintsOnlyNeededFilesWithoutWriting` excludes unrelated files and preserves existing selections.
- [x] [S-3] `TestDeclaredHostsAndExistingCopiesAreChecked` rejects declared missing and existing drifted files without demanding unrelated hosts.
- [x] [S-4] `TestOpenCodeAndKiloSupplyExplicitSetupContext` and command parity tests pass; `docs/commands.md` explains print versus write.

Evidence: `procoder test` passed all 57 Go packages, including Node-driven adapter
tests. `procoder check` reported 32 clean files and zero blocking findings.
`go run ./cmd/procoder agents --all` reported every distributed rule file current.
The JS package has no standalone test script; no standalone JS suite is claimed.

## Open questions

<!-- No unresolved questions; the user approved this scope. -->
