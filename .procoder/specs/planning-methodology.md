# planning-methodology

Status: complete

## Problem

Procoder governs how work is verified and says almost nothing about how it
is conceived. The chain it offers — spec → plan → backlog → sprint — begins
at a spec that is already written, and its controllers check that a document
is _complete_, never that the idea in it is _good_. `spec check` will pass a
thoroughly filled-in specification for the wrong feature. Nothing in the tree
helps a person get from a vague notion to a spec worth checking, and nothing
reviews a change with judgment rather than tooling: `procoder check` reads
formatting, secrets, linters and hygiene, all of which are mechanical.
Everything requiring an opinion — is this the right shape, what breaks at the
edges, would a test actually catch this regressing — is left to whoever
happens to be looking.

BMad Method (bmad-method 6.11.0, MIT, npm) has spent that effort on exactly
the half procoder skipped, and has 52k stars saying it landed. It ships an
analysis phase before planning, five review lenses with distinct stances, and
named agent personas for multi-perspective work. Meanwhile it has no commit
gate, no test harness, no formatter, no release controller, no debt ledger —
the half procoder spent its effort on.

Now, because the two are converging on the same users. A repository adopting
procoder for its gate has to leave procoder to plan, and a repository
adopting BMad to plan has nothing verifying the result before it ships. Both
groups are stitching the halves together by hand.

## Users

**A repository that wants procoder's governance and better planning.** Has
the gate, the suite, the release controller. Wants the analysis and review
depth without installing a second framework, learning a second vocabulary,
or committing a second set of artifact directories.

**A repository already running BMad.** Has planning artifacts, a
sprint-status.yaml, and a team that knows the workflow. Wants procoder's gate
and release controller without abandoning any of it, and without procoder
demanding a parallel `.procoder/backlog/` that duplicates what BMad already
tracks.

**A reviewer of either.** Needs findings that read the same however they were
produced, because a person triaging a commit should not have to know which
methodology generated the finding in front of them.

## In scope

Two tracks, deliberately separable — each ships value without the other.

**Track 1 — procoder grows the capability in its own code.**

- `procoder review` — multi-lens review over a diff, file, or document.
  Five lenses, each a distinct stance: adversarial, edge-case,
  verification-gap, structure, prose. Written in procoder's own words, not
  BMad's.
- An analysis phase ahead of the spec, under `.procoder/analysis/`, for
  getting from a notion to something `spec check` can meaningfully judge.
- Perspectives — analyst, architect, implementer, reviewer — applied as
  review stances at spec and plan time.
- Right-sizing: naming which entry point in the chain a change belongs at,
  and making every entry point reachable rather than only the seeded one.
- Every lens, perspective and phase repo-overridable from `.procoder/`,
  following D-OVERRIDE like every other domain.

**Track 2 — a repository can run the real BMad instead.**

- `[planning] method = "procoder" | "bmad"` in `.procoder/config.toml`,
  defaulting to `procoder`.
- Under `bmad`, procoder's planning controllers read and validate BMad's
  artifacts — `planning-artifacts/`, `implementation-artifacts/`,
  `sprint-status.yaml` — instead of `.procoder/specs|plans|backlog`.
- `procoder status` reports sprint state from `sprint-status.yaml`.
- `procoder doctor` reports whether BMad is installed and at what version.
- The governance backbone — gate, test, format, release, debt, scrub,
  security, docs — is untouched by the setting and runs identically in both
  modes.

## Out of scope

- Vendoring, bundling, or forking any BMad code. Track 2 reads artifacts a
  separately installed BMad produced; procoder never ships it.
- Copying BMad's prompt text, lens wording, or skill content. Track 1 is a
  reimplementation of ideas, in procoder's voice, for the licensing and
  trademark reasons under Constraints.
- Named personalities. BMad's agents are people with names; procoder's
  perspectives are stances without them — see D-3.
- Writing into BMad's artifact directories. Procoder reads and reports on
  them; BMad owns what it wrote, and a governance tool that edits the
  artifacts it judges is not a governance tool.
- Making the analysis phase mandatory. It is the answer to "I do not know
  what I am building yet", not a new tollgate for people who do.
- Any second host for track 2. BMad's own installer targets many IDEs;
  procoder reads the artifacts, which are host-independent.

## Constraints

- **Trademark.** "BMad" and "BMAD-METHOD" are trademarks of BMad Code, LLC.
  Track 1's features carry procoder names and procoder wording. Track 2 may
  name BMad only to describe interoperation — a setting value, a doctor line,
  a documentation sentence — never as the name of a procoder feature.
- **MIT attribution.** BMad is MIT licensed, so its code and text may be
  reused with the copyright notice retained. Track 1 avoids that obligation
  entirely by not copying: the ideas are not copyrightable, the expression
  is, and procoder writes its own.
- **P-CONTROL.** `procoder review` cannot review anything: the binary is not
  a language model. It prints the lens and the scope; the agent judges; the
  findings come back through the same `gitx.Finding` pipeline every other
  domain uses. Same shape as `procoder format` — the binary prints, the agent
  writes.
- **No silent green.** A lens that could not run, an artifact directory that
  could not be read, a `sprint-status.yaml` that will not parse — each says
  so and blocks, exactly as a missing linter does. A review that did not
  happen must never read as a review that found nothing.
- **Both modes answer the same questions.** Whatever the setting,
  `procoder status` says where the work stands and the close controllers say
  whether a thing is done. Only where they read the answer changes.
- **Offline.** Track 2 reads files on disk. It never invokes BMad, spawns its
  skills, or reaches the network — the gate runs on every commit.
- The setting is read once, from the existing config loader, and an
  unrecognised value is a Problem naming the line, like every other key.

## Interfaces

| Surface                                   | Behaviour                                                                                                                      |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `procoder review [paths...]`              | Prints each applicable lens and the content in scope, for the agent to judge. Exit 0.                                          |
| `procoder review --lens <name>[,<name>]`  | Runs only the named lenses. An unknown name is reported and nothing is printed.                                                |
| `procoder analyze <sub>`                  | The pre-spec phase: `brief`, `explore`, `list`, `check`. Prints documents for the agent to write, under `.procoder/analysis/`. |
| `[planning] method`                       | `"procoder"` (default) or `"bmad"`. Anything else is a Problem naming the line.                                                |
| `.procoder/review/lenses/<name>.md`       | Replaces one shipped lens. Present-and-empty is an error, not a fallback.                                                      |
| `.procoder/review/perspectives/<name>.md` | Replaces one shipped perspective, same rule.                                                                                   |
| `procoder doctor`                         | Under `method = "bmad"`, reports whether BMad is installed and its version.                                                    |
| `procoder status`                         | Under `method = "bmad"`, reports sprint state from `sprint-status.yaml`.                                                       |

## Data

Track 1 adds two directories a repository commits:

- `.procoder/analysis/<slug>.md` — analysis documents, same frontmatter
  convention as specs (`Status:`, `Created:`), so `analyze check` can refuse
  a hollow one the way `spec check` does.
- `.procoder/review/lenses/`, `.procoder/review/perspectives/` — overrides
  only. Absent means the shipped set, which lives in the binary.

Track 2 adds nothing and owns nothing. It reads, at paths BMad's own
`module.yaml` defines and its config records:

- `{output_folder}/planning-artifacts/` — PRD, spec, architecture, epics
- `{output_folder}/implementation-artifacts/sprint-status.yaml` — the
  `development_status` map, statuses `backlog`, `ready-for-dev`,
  `in-progress`, `review`, `done`, plus `action_items`
- `_bmad/config.toml` — for `output_folder`, rather than assuming its default

## Edge cases

- **`method = "bmad"` with BMad not installed.** Blocking finding naming the
  setting and the missing installation. Silently falling back to procoder's
  own chain would leave a repository believing BMad governs its planning
  while procoder quietly governed it instead.
- **`method = "bmad"` with BMad installed but no artifacts yet.** Not an
  error — a fresh install has planned nothing. Reported as "no planning
  artifacts yet", the way an empty backlog is.
- **A `sprint-status.yaml` that will not parse.** Blocking, naming the file.
  Distinct from absent.
- **A status vocabulary BMad extends.** An unrecognised status is reported by
  name rather than mapped to the nearest procoder equivalent — guessing that
  `blocked` means `open` is how a status machine quietly loses a state.
- **Both `.procoder/backlog/` and BMad artifacts present.** Under `bmad`,
  procoder reads BMad's and says once that the procoder backlog is being
  ignored, so a half-migrated repository is told rather than left to wonder
  which one the report came from.
- **A lens override that is empty or unreadable.** Blocks, naming the file —
  a repository must never believe it replaced a lens that is still running
  procoder's version.
- **`procoder review` over a binary or undecodable file.** Out of scope,
  counted out loud, exactly as the gate counts a file type it does not claim.
- **An analysis document for a spec that already exists.** Allowed. Analysis
  is not a phase you leave; a spec that turns out wrong sends you back to it.

## Failure modes

- **`_bmad/config.toml` unreadable** (permissions, or a format change):
  blocking finding naming the file. Procoder must not guess `output_folder`
  and then report on a directory the repository is not using.
- **BMad's artifact layout changes in a future version:** the version doctor
  reports is the evidence. A layout procoder cannot read is a blocking
  finding naming the version it found, not a silent empty report.
- **A lens the agent does not return findings for:** the review is
  incomplete and says so. Zero findings from a lens that ran is a valid
  answer for most lenses and an explicit re-check signal for the adversarial
  one (D-4); a lens that never ran is neither.
- **The config file itself unparseable:** the existing config leg already
  blocks; the planning method must not resolve on defaults behind that block.

## Acceptance criteria

- [ ] `procoder review` over a fixture diff prints all five lenses with the
      content in scope, and leaves every file's bytes unchanged, asserted by
      comparing a digest of the tree before and after.
- [ ] `procoder review --lens edge-case` prints exactly that lens, and an
      unrecognised name reports the name and prints no lens at all.
- [ ] A repository carrying `.procoder/review/lenses/adversarial.md` gets
      that content in place of the shipped lens; without the file it gets the
      shipped one unchanged.
- [ ] An empty `.procoder/review/lenses/adversarial.md` blocks and names the
      file rather than falling back to the shipped lens.
- [ ] `procoder analyze check` refuses a hollow analysis document — a
      section left as its template comment is not a filled section — and
      passes a filled one.
- [ ] `procoder spec check` names the analysis document a spec came from when
      one exists, and does not require one when it does not.
- [ ] `[planning] method = "bmad"` with no BMad installed produces a blocking
      finding naming both the setting and the missing installation.
- [ ] `[planning] method = "bmad"` with a fixture BMad install reports sprint
      state from `sprint-status.yaml`, with each story's status, and does not
      report from `.procoder/backlog/`.
- [ ] A `sprint-status.yaml` that will not parse produces a blocking finding
      naming the file, distinct from the finding for one that is absent.
- [ ] A status in `sprint-status.yaml` that procoder does not recognise is
      reported by name rather than mapped to a procoder status.
- [ ] `[planning] method = "nonsense"` is a config Problem naming the line,
      and the run continues on the default.
- [ ] Under `method = "bmad"`, `procoder check` produces byte-identical
      output to the same tree under `method = "procoder"`, asserted on a
      fixture — the setting governs planning and nothing else.
- [ ] `procoder doctor` under `method = "bmad"` names BMad's installed
      version, and says plainly that it is absent when it is.
- [ ] No procoder-owned feature name contains "BMad", asserted by an audit
      over the source and the command table, so the trademark boundary cannot
      erode by accident.

## Open questions

<!-- none — decisions below -->

## Decisions

- **D-1: two tracks, one spec, shipped independently.** They share a
  vocabulary and a config key, and each is useless to the other's audience.
  Speccing them together keeps the boundary honest — every capability track 1
  adds is one track 2 must not duplicate — while letting either ship first.

- **D-2: reimplement the ideas, never the expression.** BMad is MIT, so
  copying with attribution is permitted; the reason not to is that the
  trademark forbids procoder naming features after it, and a feature whose
  prompt is BMad's words but whose name cannot be is a confusing hybrid. The
  ideas — a lens with a stance, a phase before the spec — are not
  copyrightable. Procoder writes its own words and owes nothing.

- **D-3: perspectives, not personalities.** BMad's agents are named people:
  Mary the analyst, Winston the architect. That is a deliberate and effective
  choice for a chat-first tool. Procoder has no voice by design — the binary
  reports facts, the agent speaks — and giving it five named characters would
  be adopting an aesthetic rather than a capability. The capability is the
  multi-angle read; procoder takes that and leaves the cast.

- **D-4: the adversarial lens treats zero findings as a re-check signal.**
  Taken directly from BMad's stance, because it is procoder's own rule
  arriving from another direction: "no silent green" says a check that found
  nothing must be distinguishable from a check that did not run, and a
  reviewer who returns nothing is usually the second. Every other lens may
  legitimately find nothing.

- **D-5: right-sizing is about naming the entry point, not removing
  enforcement.** The premise that procoder forces every change through
  spec → plan → backlog → sprint does not survive contact with the code: no
  gate finding requires a spec, and this repository routinely lands fixes
  that never had one. Rigidity is real only inside the backlog system, where
  it is the point. So right-sizing means saying which entry point a change
  belongs at, and making each reachable directly — not tearing out
  enforcement that was never there.

- **D-6: under `method = "bmad"`, governance is untouched.** The tempting
  design is a spectrum, with the setting dialing procoder back by degrees.
  The useful one is a single seam: planning moves, governance does not. It is
  the reason a BMad repository would install procoder at all, and a criterion
  asserts the gate's output is byte-identical across the setting so the seam
  cannot drift.

- **D-7: procoder reads BMad's artifacts and never writes them.** Both
  because BMad owns what it wrote, and because a tool that edits the
  artifacts it also judges has no standing to judge them. This is the same
  rule P-CONTROL already applies to the repository's own source.
