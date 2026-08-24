# e2e-campaign

Status: open
Created: 2026-08-24

## Question

Procoder has 78 commands, twelve formatted languages and ten domains, and
almost everything we know about whether they work comes from testing them
against this repository — a Go project with procoder already installed by
the person who wrote it. What breaks for somebody who is not us, in a
repository that is not this one?

## What we know

- **Twelve languages are claimed**: Go, Python, Rust, C/C++, shell, Java,
  Kotlin, Swift, Ruby, Dart, C#, PHP. Measured by reading the `reg(...)`
  table in `internal/tools/tools.go`, not by trusting the docs.
- **78 top-level command arms** in `cmd/procoder/main.go`, counted from
  the dispatch. The suite covers their logic; almost none of it covers
  the command as a user invokes it.
- **Every bug this session found was a silent one.** A spec at fourteen
  of fourteen with two features never built. An audit blind to the
  literal shape it scanned for. A `#` read as part of a value. In each
  case the thing reported success. Nothing was noisy.
- **This repository is an unrepresentative fixture.** It has a
  `.procoder/` directory built up over a year, a populated backlog, an
  agent layer, a code index, and every tool installed. A new adopter has
  none of that, and most procoder paths branch on exactly those.
- **Two languages have no linter at all** (C#, Dart) and reach
  `lintUnlinted`; the rest reach a real tool. That branch was wrong until
  #152 and nothing outside a unit test had exercised it.

## What we do not know

- **What a genuinely empty repository does to each command.** Most were
  written against a repository that already had state. `procoder status`
  on a fresh `git init` is a different code path from the one anyone runs
  daily. Resolved by running every command against a fixture built from
  nothing.
- **Whether the twelve languages actually format, lint and test**, or
  whether some are claimed by the table and broken in practice. Resolved
  by putting real source of each language in one repository and reading
  what the gate says about it — clean, unformatted, or UNCHECKED.
- **Whether the hooks fire correctly outside this checkout.** The
  PostToolUse and PreToolUse payloads are shaped by the host; procoder
  parses them. Resolved by feeding each hook a real payload.
- **Whether the docs match the binary.** `docs/commands.md` describes
  78 commands and has been edited by hand every time one changed.
  Resolved by running what the docs claim and comparing.
- **Which parts genuinely need a real GitHub repository.** `ci --runs`,
  `copilot-leak`, the release job and `docs --external` Pages checks all
  reach the API. A local fixture cannot answer for them, and pretending
  it can is how a green sweep hides the half that was never run.

## Options

- **A local fixture only.** Fastest, no external footprint, and covers
  every offline command — which is most of them. Cannot answer for
  anything that reaches GitHub, and would leave that gap unreported
  unless it is stated as loudly as the passes.
- **A real GitHub repository under the user's account.** Covers
  everything including CI, the release job, `ci --runs` and Copilot
  review capture. Costs a repository that exists afterwards and has to
  be cleaned up, and publishes something under their name.
- **Local fixture plus a real repository only for the GitHub-dependent
  commands.** Most coverage per unit of external footprint, at the cost
  of running the campaign in two environments and keeping straight which
  findings came from which.

## Recommendation

Local fixture first, and say plainly which commands it cannot answer for
rather than counting them as passed. That covers the large majority of
the 78 and needs nothing outside this machine.

The GitHub-dependent set — `ci --runs`, `copilot-leak`, the release job,
Pages health — is a real gap that a local fixture cannot close, and the
choice to open a repository is the user's rather than mine: it creates
something under their account that outlives the test. Ask before that
phase rather than deciding it here.
