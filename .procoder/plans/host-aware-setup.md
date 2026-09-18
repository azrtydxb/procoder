# Host-aware setup

## Goal

Implement .procoder/specs/host-aware-setup.md without unrelated changes.

## Architecture

Host selection is resolved at the CLI/API boundary using request context.
Portability prints additive declarations and validates only declared or existing
copies. The tool installer remains separate from integration generation.

## Constraints

Do not infer hosts from directories, default to all, silently write generated
rules, remove integrations, or modify unrelated pending decisions.

## Task 1: Select hosts at setup boundaries

Files: internal/host/setup.go, internal/host/host.go, cmd/procoder/main.go,
cmd/procoder/flags.go, internal/initcmd/gitignore.go.

Interfaces: host.Setup returns selected names, installer execution and errors.

- [x] Parse strict setup flags; resolve explicit adapter context; forward
      request environments; restrict ignore writes to selected hosts. Verify with
      go test ./internal/host ./internal/initcmd ./cmd/procoder.

## Task 2: Print and validate additive integrations

Files: internal/portability/selection.go, internal/portability/portability.go,
internal/portability/drift.go, .procoder/hosts.json.

Interfaces: portability.Agents accepts explicit selected names and prints content.

- [x] Read declared selection; print its union with requested hosts; include
      existing copies; validate declared missing files; opt this distribution into all.
      Verify with go test ./internal/portability.

## Task 3: Connect setup guidance and verify

Files: .opencode/plugins/procoder.mjs, .kilo/plugin/procoder.js,
pi-extension/index.mjs, commands/init.md, commands/agents.md, their command twins,
docs/commands.md, docs/portability.md, relevant Go regression tests.

Interfaces: PROCODER_HOST is carried in the request environment.

- [x] Propagate explicit context; document unknown-host choice and reviewed
      writes; run focused tests, procoder test, procoder review and procoder check.

## Validation

Host selection tests, CLI/API regression tests, portability and init tests,
procoder test, procoder review, procoder check. Record actual outcomes in the
completion report; leave unrelated decisions untouched.

Evidence: focused host/init/portability/CLI tests passed. `procoder test` passed
57 Go packages; its standalone JS runner was not run because no script exists.
Adversarial and edge-case review identified and fixed store-boundary violations
and dangling host-declaration symlinks. `procoder check` reported 32 clean files,
zero unformatted or unchecked files, and zero blocking findings. Existing
pending decisions and advisory lint findings remain outside this task.
