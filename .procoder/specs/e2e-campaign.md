# e2e-campaign

Status: complete

## Problem

Almost everything procoder knows about whether it works comes from
testing it against this repository — a Go project, with procoder
installed by the person who wrote it, carrying a year of `.procoder/`
state, every tool present, an agent layer, a populated backlog and a
built code index. Most of procoder's branches turn on exactly those
conditions, so the path a new adopter takes is the path least exercised.

Twelve languages are claimed and two of them reach no linter at all. 78
command arms exist in the dispatch and the suite covers their logic
rather than their invocation. The docs describe all of it and are edited
by hand each time something changes.

And every defect found this session was silent: a spec at fourteen of
fourteen with two features never built, an audit blind to the literal
shape it scanned for, a `#` read as part of a value. None announced
itself. A campaign that only runs healthy code would have found none of
them, because each one reported success.

## Users

**Someone adopting procoder tomorrow.** Runs `procoder init` in a
repository that is not Go, has no `.procoder/`, and has half the tools
missing. Needs the first run to be honest about what it checked.

**A repository that is not this one.** Multi-language, with CI that
predates procoder and a test suite in something other than `go test`.

**This project.** Needs to know which of its 78 commands are exercised
by anything at all, and which have only ever been run by their author.

## In scope

- [S-1] A fixture repository carrying real, compilable source in each of
  the twelve languages procoder claims to format, with a test suite, CI
  workflow, docs and dependency manifests — built from `git init`, not
  copied from here.
- [S-2] A clean pass: every command that runs offline, invoked against
  the healthy fixture, with each verdict recorded. A finding against
  correct code is a defect.
- [S-3] A broken pass: the fixture seeded with one deliberate defect per
  class procoder claims to catch — unformatted source, a lint finding, a
  secret, a SAST finding, a vulnerable manifest, a conflict marker, an
  oversized file, AI attribution, a debt marker with no trigger, drifted
  agent rules, a broken doc reference. Each must be caught and named.
- [S-4] Every hook exercised with a real payload on stdin —
  SessionStart, PostToolUse, PreToolUse, Stop — and its output parsed as
  the host would parse it.
- [S-5] The GitHub-dependent commands against a throwaway public
  repository: `ci --runs`, `copilot-leak`, `docs --external` Pages
  health, and a real tagged release through the workflow.
- [S-6] Every command's documented behaviour in `docs/commands.md`
  compared against what the binary does, with each disagreement recorded
  as a finding against one or the other.
- [S-7] The security domain specifically: a planted secret, a SAST
  finding and a known-vulnerable dependency, each verified to block at
  the documented severity.
- [S-8] Every finding fixed with a regression test that fails without
  the fix, and the whole campaign re-run until a pass produces nothing
  new.
- [S-9] The fixture and the throwaway repository removed when the
  campaign closes, and the removal verified.

## Out of scope

- Testing the hosts themselves. Whether Cursor writes a well-formed
  PostToolUse payload is Cursor's business; procoder's half is parsing
  what arrives and refusing what does not.
- Performance benchmarking. `procoder bench` exists and has its own
  discipline; how fast the gate runs is not what this campaign answers.
- Rewriting anything the campaign finds into a redesign. A defect gets
  the smallest fix that closes it and a test that proves it; anything
  larger becomes an issue.
- The twenty-one legacy specs that carry no scope ids. Named already,
  deliberately untouched.
- Windows and Linux as execution environments. CI covers all three; this
  campaign runs on this machine, and platform-specific findings are for
  CI to catch as it already does.

## Constraints

- **A finding is a finding whoever produced it.** Defects in procoder
  and defects in the fixture are recorded separately, because a fixture
  bug reported as a procoder bug wastes the fix.
- **No silent green, applied to the campaign itself.** A command that
  could not run — missing tool, unsupported platform — is recorded as
  NOT RUN and never counted among the passes. The campaign's own report
  must not commit the failure it is looking for.
- **The broken pass proves the clean pass meant something.** A gate that
  finds nothing on healthy code and also nothing on a planted defect is
  not working; it is silent. Every claim of "clean" is paired with the
  defect that must break it.
- **P-CONTROL holds throughout.** Nothing procoder is asked to do here
  may write to the fixture; where a command prints content for an agent
  to write, the campaign writes it and says so.
- **The throwaway repository is public, is named as a fixture, and is
  deleted at the end.** Its existence is temporary and its purpose is
  legible to anyone who finds it.
- Every fix keeps the gate green on THIS repository at the moment it is
  committed. The campaign does not get to leave the tree broken between
  sprints.

## Interfaces

| Surface                                | Behaviour                                                           |
| -------------------------------------- | ------------------------------------------------------------------- |
| `.procoder/analysis/e2e-campaign.md`   | The analysis this spec came from.                                   |
| A fixture repository outside this tree | Built by script so it can be destroyed and rebuilt identically.     |
| A throwaway public GitHub repository   | Holds the fixture for the GitHub-dependent phase; deleted at close. |
| The campaign report                    | One row per command: PASS, FINDING, or NOT RUN with the reason.     |

## Data

Nothing is stored in this repository except the analysis, this spec, its
backlog, and any regression tests the findings produce. The fixture lives
outside the tree and is rebuilt from a script rather than committed —
committing a repository full of deliberate secrets and vulnerable
manifests into this one would trip procoder's own gate, which is the
correct behaviour and an inconvenient place to keep a fixture.

## Edge cases

- **A tool procoder wants is not installed here.** Recorded NOT RUN
  naming the tool, never counted as a pass and never as a procoder
  defect — the machine is the gap.
- **A language whose formatter is present but whose linter is not.** The
  two verdicts differ and both must be recorded; this is the shape that
  produced the C#/Dart bug.
- **A command that is interactive or asks for a human.** `procoder ask`
  writes to `QA.md` when no person is present; the campaign takes that
  path and asserts the file, rather than pretending to answer.
- **A command that is destructive.** `self-upgrade` replaces the binary;
  it is exercised against a copy, never against the one running the
  campaign.
- **The fixture's own CI failing for fixture reasons.** Recorded as a
  fixture defect and fixed there; only a failure caused by procoder is a
  procoder finding.
- **A finding that is really a documentation error.** Recorded against
  the docs, and fixed in the docs, rather than changing behaviour to
  match a sentence somebody wrote.

## Failure modes

- **A defect that the campaign plants and procoder does not catch.**
  This is the campaign's whole purpose and the most likely finding,
  given every bug this session was of exactly that shape.
- **A command that cannot be invoked at all** — a dispatch arm with no
  reachable path. Recorded as a finding, since a command that exists
  only in the switch is a command the docs are lying about.
- **A fix that breaks another command.** The campaign is re-run whole
  after each round rather than only around the fix, which is what makes
  it a loop rather than a sweep.
- **The campaign passing because it did not look.** Each phase states
  what it did NOT cover, and the final report carries those alongside
  the passes.

## Acceptance criteria

- [ ] [S-1] A script builds a fixture repository from `git init` alone,
      carrying compilable source in all twelve claimed languages plus a
      test suite, CI workflow, docs and manifests, and rebuilding it
      twice produces identical trees.
- [ ] [S-2] Every offline command is invoked against the healthy fixture
      and its verdict recorded; any finding raised against correct code
      is reported as a procoder defect with the command that raised it.
- [ ] [S-3] One deliberate defect per class procoder claims to catch is
      planted, and each is caught and named by the command that owns it;
      any that is not caught is reported with the command that missed it.
- [ ] [S-4] Each of SessionStart, PostToolUse, PreToolUse and Stop is fed
      a real payload on stdin, and its output parses as the host expects
      — JSON envelope where the host wants one, raw text where it does
      not.
- [ ] [S-5] `ci --runs`, `copilot-leak`, `docs --external` and a tagged
      release are run against a throwaway public repository, and each
      either passes or is reported with what it did.
- [ ] [S-6] Every command documented in `docs/commands.md` is invoked and
      its actual behaviour compared with the documented behaviour, with
      each disagreement recorded against the docs or the binary by name.
- [ ] [S-7] A planted secret, SAST finding and vulnerable dependency each
      block at the documented severity, and each stops blocking when the
      documented configuration relaxes it.
- [ ] [S-8] Every finding has a fix and a regression test that fails
      without it, and a final full run of both passes produces no finding
      that was not already recorded and fixed.
- [ ] [S-9] The fixture directory and the throwaway repository are both
      gone at close, verified by their absence rather than by assertion.

## Open questions

<!-- none — the two that mattered are decided below -->

## Decisions

- **D-1: two passes, clean then broken, and the broken one is the
  point.** A campaign that only runs healthy code answers "does procoder
  complain about correct work", which is worth knowing and is not the
  question that has been biting. Every defect this session — the spec at
  fourteen of fourteen, the blind audit, the comment read as a value —
  reported success. Only a planted defect that goes uncaught reveals
  that class, so every "clean" verdict is paired with the defect that
  must break it.

- **D-2: a real throwaway public repository for the GitHub-dependent
  set.** `ci --runs`, `copilot-leak`, Pages health and the release job
  reach the API, and a local fixture cannot answer for them. Calling
  them covered on the strength of a local run would be the campaign
  committing the exact failure it is hunting. Public rather than
  private: Actions minutes are free there, and this campaign runs CI
  many times.

- **D-3: the fixture is built by script and never committed here.** It
  carries deliberate secrets and vulnerable manifests, and committing it
  would trip this repository's own gate — correctly. A script also makes
  the fixture reproducible, which a copied directory is not, so a
  finding can be reproduced from scratch rather than from a state
  somebody happened to be in.

- **D-4: findings are attributed before they are fixed.** A fixture bug
  fixed in procoder is a change that makes procoder wrong to match a
  broken test, and a procoder bug fixed in the fixture hides the defect
  entirely. Each finding names which side it belongs to first.
