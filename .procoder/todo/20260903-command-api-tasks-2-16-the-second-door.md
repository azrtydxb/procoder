# command-api Tasks 2-16: the second door

Status: closed
Created: 2026-09-03

## Description

Tasks 2 through 16 of `.procoder/plans/command-api.md`: the envelope, the
in-process runner, the typed results, the confirmation, `procoder serve`,
the client and its fallback, per-root serialisation, jobs, the exec
socket, warm-state eviction, auto-start, the config keys and the question
`init` asks, the parity table, and the three line-oriented domains.

Done means every one of the 47 commands can be called over a local socket
and answers exactly what the binary answers, with the four that execute
what a repository declared reachable only behind their own door.

## Acceptance criteria

- [x] Every command answers byte-identically at both doors:
      `TestParityAcrossEveryCommand` compares 48 commands on stdout,
      stderr and the exit code.
- [x] The environment travels in the request, so one daemon serves two
      hosts correctly: `TestParityVariesTheEnvironmentNotJustThePayload`.
- [x] The four executing commands are refused on the work socket, and the
      transport cannot run anything at all — asserted by
      `TestExecutingCommandsRefusedOnWorkSocket` and by
      `TestClientTransportExecutesNothing`.
- [x] Concurrent requests against one repository do not lose writes:
      `TestPerRootSerialisation`, under `-race`.
- [x] A daemon that cannot be reached costs no answer:
      `TestDeadSocketCostsNothing`, `TestVersionSkewFallsBackInProcess`.
- [x] Long-running commands outlive their connection:
      `TestJobSurvivesItsConnection`.
- [x] `go test ./...` passes and `procoder check` reports 0 blocking.

## Evidence

`go test ./...` — 57 packages ok, 0 FAIL.

`procoder check` — 0 blocking, on every one of the sixteen commits.

`go test ./cmd/procoder/ -run TestParityAcrossEveryCommand -v` — 48
subtests PASS, one per command read out of the usage text.

Live, against a real daemon: a `check` request over the socket returned
exit 0 with 18 findings as typed data and the human bytes unchanged; the
socket came back `srw-------`; `self-upgrade` over the work socket
returned exit 2 naming #201 and the exec socket's path; five session-start
hooks fired at once left exactly one daemon.

Commits 6747c9f through 8c1d5df on feat/command-api.
