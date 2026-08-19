# deps-freshness

Status: draft

## Problem

The security pass catches vulnerable dependencies; nothing reports
stale ones or their licenses. Real teams review freshness weekly —
updates get cheaper the smaller they are — and license conflicts are
found at the worst possible time when nobody looks. Today the agent
would shell out ad hoc, differently every time.

## Users

- Pascal doing the weekly hygiene pass: one report of what is behind
  and by how much.
- The agent judging whether an update belongs in the current sprint.

## In scope

- `procoder deps` — the freshness report, per ecosystem, native tools
  only:
  - Go: `go list -u -m all` — module, current, available.
  - JS: `npm outdated --json` (via the lockfile's manager where it
    supports the same output; otherwise npm) — package,
    current/wanted/latest.
  - Rust: `cargo outdated` when installed; honestly NOT checked when
    not (it is a plugin, procoder installs nothing into toolchains).
  - Python: `pip list --outdated --format json` when a project
    virtualenv is active/present; NOT checked otherwise, said with the
    reason.
    Counts summarized (N modules behind, M majors); report-only, exit 0
    with findings, 1 only when an ecosystem's tool errored.
- Licenses, honest scope: Go via `go-licenses report` when installed
  (recommended by doctor for Go repos with dependencies but not
  required); everything else answers "licenses NOT checked — no
  canonical no-install tool for this ecosystem". Never a fake
  all-clear.
- Findings capped per ecosystem at 30 with an "…N more" line.

## Out of scope

- Applying updates, editing manifests, or lockfile regeneration.
- License policy enforcement (allow/deny lists) — reporting only.
- Private-registry authentication handling beyond what the native
  tools inherit from the environment.
- Blocking anything, ever — freshness is judgment.

## Constraints

- Pure Go stdlib; package internal/deps; tools resolved as usual.
- P-CONTROL: read-only.
- Honesty rule: a tool that errored yields NOT checked for that
  ecosystem, never an empty (clean-looking) section.
- Network use is inherent to the underlying tools and stated in the
  skill doc; a timeout of 120s per tool with the hung-tool message.

## Interfaces

- `procoder deps` (no flags in v1).
- Usage text, docs.Commands, docs site, commands/deps.md skill +
  OpenCode twin; doctor recommends go-licenses on Go repos (advisory
  wording inside the deps output, not a doctor GAP).

## Data

- No stored state.

## Edge cases

- Repo with no manifests at all → "no dependency manifests in this
  repository", exit 0.
- `go list -u` on a module with replace directives → rows print as the
  tool reports them (no reinterpretation).
- npm workspaces → npm outdated already aggregates; reported as-is.
- Everything current → each section says up to date; summary zero.
- Offline → the underlying tool errors; NOT checked with its first
  error line.

## Failure modes

- Tool missing → NOT checked naming the install (cargo-outdated,
  go-licenses).
- Tool timeout → NOT checked, "gave no answer in 120s".
- Unparseable JSON from npm/pip → NOT checked with the parse reason —
  never a silently empty table.

## Acceptance criteria

- [ ] On procoder's own repo, `procoder deps` prints a Go section with
      real freshness rows (or an explicit up-to-date line) and a
      licenses line that is either a go-licenses report or an honest
      NOT-checked with the install hint.
- [ ] On a fixture npm project with a pinned old dependency, the JS
      section names it with current and latest versions.
- [ ] A repo with no manifests answers with the no-manifests line and
      exit 0.
- [ ] A missing optional tool (cargo-outdated) yields NOT checked
      naming the tool — verified by test with a stub PATH.
- [ ] Parse tests cover npm outdated JSON and go list -u -m output
      shapes, including the everything-current case.

## Open questions

<!-- none — decisions recorded above -->
