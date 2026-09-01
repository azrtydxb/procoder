# ci-that-procoder-writes

Status: complete

## Problem

Procoder governs a repository's commit gate and says nothing about its
CI. The two-tier model it is built on — the gate answers about the
change, CI answers about the tree — only holds where somebody has
written the second tier by hand. In this repository that happened,
because the person writing the checks also owned the workflow. Every
other repository adopting Procoder gets the gate and nothing else, which
means the whole-tree checks — `check` over every tracked file,
`security --deep`, `docs --external`, `maintain`, `debt`, `deps` — run
nowhere at all.

The domain that polices CI cannot see this. `ciops.Check` opens
`.github/workflows`, finds nothing, and returns `nil` with the comment
"no workflows, nothing to hold to CI discipline". A repository with no
CI is told it is clean. That is a silent green sitting inside the domain
whose subject is CI, and it is the same failure the checks-that-run-
themselves work removed everywhere else: a check that protects only the
people who already knew to run it.

Now, because Procoder has just finished making its own checks fire on
lifecycle events rather than on being remembered. The gate half of that
is portable and shipped. The CI half is a workflow file in one
repository.

## Users

**A repository adopting Procoder.** Runs `procoder init`, gets a gate,
and has no way to know a second tier exists. Needs to be told what is
missing and handed the steps, in the shape its own stack requires — not
a Go workflow it has to translate.

**The maintainer of a governed repository.** Has an existing workflow
they did not write with Procoder and will not hand over. Needs to know
which whole-tree checks their CI omits, by name, and to add them
piecemeal without adopting a generated file wholesale.

**This repository.** Its `ci.yml` is both the working CI and the worked
example. Whatever is emitted has to be the thing that already runs here,
or the example is a fiction — and the emitter is the only way to keep
those two in step.

## In scope

- [S-1] `procoder ci --emit` prints a complete workflow for the repository it
  is run in — the steps only, to stdout, for the agent or the person to
  review and write. The binary writes no file.
- [S-2] Emission is modular: one block per concern (suite, gate over the tree,
  security, docs, whole-tree pass, release), and a block is emitted only
  where the repository has something for it to check.
- [S-3] Emission is adaptable: which ecosystems appear comes from what the
  repository contains, and the policies inside the steps come from
  `.procoder/config.toml`, so an emitted workflow enforces the gates that
  repository actually configured rather than Procoder's defaults.
- [S-4] `.procoder/ci/<block>.yml` replaces any single emitted block, the same
  way every other domain reads its rules from `.procoder/` (D-OVERRIDE).
  A repository with no such file gets the built-in block unchanged.
- [S-5] `procoder ci` reports a repository with no workflows at all, instead of
  returning nothing.
- [S-6] `procoder ci` reports, by name, the whole-tree checks an existing
  workflow does not run.
- [S-7] Host rendering is a seam: the block set is host-independent and GitHub
  Actions is one renderer of it, so adding another host later is a
  renderer rather than a rewrite.

## Out of scope

- Any host other than GitHub Actions in this change. The seam exists;
  the second renderer does not.
- Writing, patching or merging into an existing workflow file. Procoder
  prints; the agent writes. A tool that edits CI is a tool that can
  disable CI.
- Refreshing pinned action SHAs from the network. The pins ship as a
  table and are updated the way `.github/tool-versions.env` is.
- Emitting the deploy or publish half of anybody's pipeline. Procoder
  knows what a repository should check, not where it ships.
- Reproducing this repository's own release job, which is specific to
  shipping five platform binaries and a checksum manifest.

## Constraints

- **P-CONTROL.** The binary prints and never writes, including here. The
  emitted YAML goes to stdout.
- **No silent green.** An absent workflow directory, an unreadable one,
  and a workflow missing a tier are three different answers, and none of
  them is silence.
- **Self-consistency.** The emitted workflow must pass `procoder ci`'s
  own hygiene rules: every action pinned by SHA, every job carrying a
  timeout, concurrency cancelling in progress. A generator whose output
  fails the checker in the same binary is a bug in one of the two.
- **The emitted workflow must be the one that runs here.** If this
  repository's `ci.yml` and the emitter disagree, the example is a
  fiction; a test holds them together.
- **Offline.** Emission is a local render. No network, so it works in
  `init` on a fresh clone.
- Emission must stay inside the gate's latency budget when it runs as a
  finding, so the check reads files and never executes tools.

## Interfaces

| Surface                            | Behaviour                                                                                   |
| ---------------------------------- | ------------------------------------------------------------------------------------------- |
| `procoder ci --emit`               | Prints the whole workflow for this repository to stdout. Exit 0.                            |
| `procoder ci --emit --host <name>` | Selects the renderer. An unknown name is reported and nothing is printed.                   |
| `procoder ci`                      | Existing hygiene report, plus: no workflows at all, and whole-tree checks a workflow omits. |
| `.procoder/ci/<block>.yml`         | Replaces one emitted block. Present-and-empty is an error, not a fallback.                  |
| `[ci]` in `.procoder/config.toml`  | Which optional legs the emitter includes — the platform matrix in particular.               |

## Data

Nothing new is stored. The emitter reads:

- the repository tree, for which ecosystems exist (`go.mod`,
  `package.json` with a test script, `Cargo.toml`, `composer.json`,
  `pom.xml`/`build.gradle`, pytest markers) — the same detection
  `testrun` already performs, reused rather than re-implemented;
- `.procoder/config.toml`, for the policies the steps enforce;
- `.procoder/ci/`, for block overrides;
- a table of pinned action SHAs compiled into the binary, alongside the
  version table CI already keeps.

Output is stdout. The only file that changes is the one a person writes.

## Edge cases

- **An existing workflow written by somebody else.** Reported against,
  never rewritten. The finding names missing checks; it does not propose
  a diff.
- **A repository with several ecosystems.** Every detected one emits its
  block; the order is stable so two runs produce identical bytes.
- **A repository with no ecosystem Procoder recognises.** The whole-tree
  gate block still emits — `procoder check` applies to any tree — and
  the suite block does not.
- **A repository whose CI is not GitHub Actions.** `.github/workflows`
  absent must not be read as "no CI" when `.gitlab-ci.yml` exists; the
  finding says which hosts were looked for.
- **An override file that is empty or unparseable.** Blocks, naming the
  file. Falling back to the built-in would mean a repository believing it
  had replaced a block that still runs Procoder's version.
- **A workflow that runs the right commands under a different job or
  step name.** Detection matches the commands, not the names, or every
  repository that renamed a step is told it is missing a check it runs.
- **A pinned SHA that no longer resolves.** Emission is offline and
  cannot know; the emitted comment carries the tag alongside the SHA so
  the staleness is visible to a reader and to `procoder deps`.

## Failure modes

- **`.github/workflows` unreadable** (permissions): blocking finding
  naming the directory. Distinct from absent.
- **The repository is not a git repository**: the ecosystem detection
  still works from the filesystem; nothing requires git.
- **`.procoder/config.toml` unparseable**: the existing config leg
  already blocks; the emitter must not run on defaults behind that block.
- **An override block is present but the built-in it replaces no longer
  exists** (a renamed block after an upgrade): named, so an override
  cannot silently stop applying.
- **stdout is a pipe that closes early**: emission is a single write of a
  complete document; a partial workflow must never be produced, because a
  truncated YAML that still parses is a CI that silently checks less.

## Acceptance criteria

- [x] [S-5] A repository with no workflow files at all produces a finding
      naming the whole-tree tier as missing, where `ciops.Check`
      previously returned nil.
- [x] [S-5] A repository whose `.github/workflows` is unreadable produces a
      different, blocking finding that names the directory — not the
      absent-CI one.
- [x] [S-1] [S-2] [S-3] `procoder ci --emit` in a Go repository prints a workflow
      containing the whole-tree steps (`check` over tracked files,
      `security --deep`, `docs --external`, `maintain`, `debt`, `deps`)
      and a Go suite step.
- [x] [S-1] The emitted workflow runs `procoder docs --external` and not the
      bare `procoder docs`: the offline half already rides the gate, so
      emitting it again in CI would repeat the commit's answer while
      leaving link rot — the only part CI can see — unchecked.
- [x] [S-3] The same command in a repository with only a `composer.json` and
      PHP sources prints the PHP suite step and no Go step.
- [x] [S-1] The emitted workflow passes `procoder ci`'s hygiene rules:
      asserted by feeding the emitter's own output back into
      `ciops.Check` and requiring no findings.
- [x] [S-1] `procoder ci --emit` leaves every file's bytes unchanged, asserted
      by comparing a digest of the tree before and after.
- [x] [S-4] A repository carrying `.procoder/ci/whole-tree.yml` gets that
      content in place of the built-in block, and a repository without
      the file gets the built-in block unchanged.
- [x] [S-4] An empty `.procoder/ci/whole-tree.yml` blocks and names the file,
      rather than falling back to the built-in.
- [x] [S-6] A workflow that runs `procoder check` but not `procoder debt` is
      told that `debt` is missing and is not told that `check` is.
- [x] [S-6] A workflow that runs the whole-tree commands under step names
      Procoder did not choose produces no missing-check finding.
- [x] [S-7] `--host` with an unrecognised name reports the name and prints no
      workflow.
- [x] [S-2] This repository's own `ci.yml` contains every whole-tree step the
      emitter emits for it, asserted by a test, so the shipped example
      and the running CI cannot drift apart.

## Open questions

<!-- none — the two that mattered are resolved below, from measurement -->

## Decisions

- [D-1] the emitted matrix is ubuntu and windows; macOS is opt-in.
  Measured on this repository: the Windows leg has caught four recorded
  bugs — a rooted literal used as a test root (PR#134, #136), Windows path
  length limits (PR#34), `filepath.Join` in printed paths and assertions
  (PR#33), and launcher resolution under Git Bash, MSYS and Cygwin. Each
  was invisible to Linux and macOS. `LESSONS.md` records the most recent
  as "Missed by: the local suite — macOS and Linux call `/repo` absolute,
  Windows does not". No recorded incident shows macOS catching what Linux
  did not, and development here happens on darwin, so the local suite
  covers macOS continuously and the CI leg largely repeats it. `[ci]
platforms` adds it back for repositories where that is not true.

- [D-2] action versions are pinned by SHA from a compiled-in table.
  Emitting floating tags would produce a workflow that fails
  `procoder ci`'s own pinned-actions rule, which is incoherent from a
  single binary. The table is maintained like `.github/tool-versions.env`;
  the emitted line carries the tag as a comment so a reader can see what
  the SHA claims to be.

- [D-3] the whole-tree checks are not filtered by changed files.
  Proposed in issue #142 as an optimisation. Measured on the run that
  prompted it: `maintain`, `debt` and `deps` together cost 1 second and
  `docs --external` costs 6, against a 185-second job whose largest single
  step is `procoder check` at 100. The saving does not pay for the cost:
  these checks answer about time as much as about content — a link rots
  when somebody else's server changes, a dependency falls behind when
  upstream ships — so gating them on the local diff means running them on
  the days they are least likely to have news. It would also collapse the
  two tiers that `TestAnUntouchedMarkerIsCIsNotTheGates` exists to keep
  apart.
