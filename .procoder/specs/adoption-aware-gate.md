# adoption-aware-gate

Status: complete

## Problem

Clone a third-party repository, make a two-file change, and commit: procoder
blocks it with nineteen findings, seventeen of which are about procoder
rather than about the change.

Reported against a clone of `Koenkk/zigbee-herdsman-converters` (#172),
whose own gate was green — `tsc --noEmit` clean, `biome check` clean, 29/29
tests passing. What procoder said instead:

- **Twelve agents findings.** Every host's rule file "missing", and the
  repository's own `AGENTS.md` and `.github/copilot-instructions.md` —
  written by that project, for that project — declared "drifted from
  AGENTS.md" and demanded a rewrite.
- **Two release findings.** README and CHANGELOG must carry version
  `26.99.0`. That project generates its changelog with release-please; its
  version scheme is not procoder's to satisfy.
- **Two formatting findings.** Both changed files "unformatted", while
  Biome — the formatter that repository declares — reports them clean.
- **One lint finding.** "typescript-eslint is not installed; run `procoder
init`", in a repository that chose Biome. Installing ESLint there would
  be a wrong change to somebody else's project.
- **One secret false positive**, on a line the commit does not touch: a
  `const` whose name ends in `_STORE_KEY`, assigned the camel-cased spelling
  of that same name. No credential — a key into a store — and 2,500 lines
  from the diff.

  It is not quoted verbatim here, and the reason is the point: writing that
  line into this file made procoder's own gate refuse the commit carrying
  the specification of the bug. The false positive reproduces, in this
  repository, against a document describing it.

The gate is all-or-nothing, so the only way through was `--no-verify`,
which also disables the checks that were worth having. The reporter's own
words: the failure mode is that `--no-verify` becomes muscle memory, and
then the gate protects nothing.

This is not an edge case. Fork, patch, PR is a routine workflow, and it
happens on every clone of every upstream project.

## Users

**Somebody contributing upstream.** Has procoder installed globally and is
working in a repository that never adopted it. Wants the parts that are
true anywhere — do not commit a secret, do not commit a conflict marker —
and none of procoder's house style.

**A repository that adopted procoder.** Must lose nothing. Every check it
gets today it still gets, because that repository asked for them.

**Somebody evaluating procoder.** Clones it, tries it on a real project,
and currently meets nineteen findings that read as arrogance.

## In scope

- [S-1] procoder decides whether a repository has adopted it, from evidence
  in the repository itself: a `.procoder/` directory, or an `AGENTS.md`
  that names procoder.
- [S-2] In a repository that has NOT adopted procoder, the gate runs only
  the checks that are true regardless of house style: secrets, oversized
  files, conflict markers, junk files, and AI attribution in the commit
  message.
- [S-6] In a repository that has NOT adopted procoder, the checks that read
  file CONTENT — secrets and conflict markers — see only the lines this
  commit added or changed, never the whole file. In somebody else's
  repository the only code that is mine to answer for is the code I wrote.
  This is what removes the last finding in the report: a constant named
  `..._STORE_KEY` on line 4,423 of a file whose diff sits 2,500 lines away.
  Checks that are about a file's existence rather than its contents —
  oversized, junk — are unchanged, because a file this commit introduces
  is this commit's, all of it.
- [S-3] In a repository that has NOT adopted procoder, the gate runs none
  of the domains that encode procoder's own conventions: the agent layer,
  release and documentation hygiene, procoder's templates, the planning
  chain, formatting, linting, maintainability, debt, and the suite.
- [S-4] procoder never claims a file it did not write. An `AGENTS.md` or
  `.github/copilot-instructions.md` that procoder did not generate is not
  "drifted" — it is somebody else's file, and the word is wrong.
- [S-5] The verdict is visible and overridable. The gate says which mode it
  ran in, and a repository can force either one.

## Out of scope

- Hunk-scoping the content checks in an ADOPTING repository. There the
  argument cuts both ways: a pre-existing secret in a file you touched is
  not your commit's fault, and it is still a secret in your repository, and
  a project that adopted procoder asked to be told. Non-adopting
  repositories have no such tension — the file is not yours — so S-6 is
  scoped to them and the harder question stays open.
- Deferring to a repository's own formatter or linter when it declares one
  (`biome.json`, `.prettierrc`, `eslint.config.js`). Worth doing, and
  larger than it looks: it means procoder reading another tool's config to
  decide whether to stay quiet. In a non-adopting repository this change
  makes it moot by not running those domains at all.
- Any change to what an adopting repository sees. This is about where
  procoder applies its rules, not which rules it has.

## Constraints

- **An adopting repository loses nothing.** Whatever it is told today it is
  told after this change, in the same words, blocking in the same way.
- **Adoption is decided from the repository, never from the environment.**
  Not a flag on the binary, not a variable in a shell — a contributor's
  machine looks the same in both repositories and the answer must not.
- **Absence of evidence is not adoption.** A repository that shows no sign
  of having adopted procoder is treated as not having adopted it. The
  failure direction is saying less about somebody else's code, not more.
- **The universal checks stay blocking.** A secret is a secret in any
  repository. Reducing scope must not reduce those to advice.
- **No silent green.** The gate says which mode it ran in. A quieter gate
  that does not say it is quieter is indistinguishable from a clean one.

## Interfaces

| Surface                                   | Behaviour                                                                                                         |
| ----------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `.procoder/` present                      | Adopted. Everything runs, exactly as today.                                                                       |
| `AGENTS.md` naming procoder               | Adopted. Same.                                                                                                    |
| Neither                                   | Not adopted. Universal checks only, and the gate says so.                                                         |
| `[gate] scope` in `.procoder/config.toml` | `adopted` or `universal`, forcing either mode. Only readable where `.procoder/` exists, which is itself adoption. |
| `PROCODER_GATE_SCOPE`                     | Same two values, for a repository that cannot carry config — a fork somebody is about to submit upstream.         |

## Data

Nothing new is stored. Adoption is computed from two paths that already
exist, on every gate run, from the filesystem.

## Edge cases

- **`AGENTS.md` exists and does not mention procoder.** Not adoption. That
  file belongs to whoever wrote it, and S-4 says procoder does not claim
  it.
- **`.procoder/` exists but is empty**, left by an abandoned experiment.
  Adoption: somebody put it there, and the failure direction for a
  directory bearing procoder's own name is to run.
- **A monorepo whose root adopted procoder, committed from a subdirectory.**
  The repository root is what decides, because that is what `.procoder/`
  belongs to.
- **A repository that adopted procoder AND declares Biome.** Adopted, so
  everything runs, formatting included. Resolving that conflict is the
  out-of-scope deference work; this change must not silently pick a side.
- **Not a git repository at all.** Adoption is still decided from the
  working directory, because the gate can be run there.
- **`AGENTS.md` unreadable.** Not adoption, and the reason is said. An
  unreadable file is not evidence of anything.

## Failure modes

- **An adopting repository quietly loses checks.** The worst outcome here
  by a distance, because it is silent. Held by a test that runs the whole
  gate over a fixture with `.procoder/` and requires every domain still to
  report.
- **A non-adopting repository still gets house rules**, because one domain
  was missed when the list was written. Held by a test that enumerates the
  domains rather than sampling them.
- **The mode is decided differently by two callers** — `procoder check`
  saying one thing and the pre-commit hook another. Held by both going
  through one function.
- **Somebody reads the quieter gate as a clean one.** Held by S-5: the gate
  names the mode it ran in, every time.

## Acceptance criteria

- [ ] [S-1] A repository with `.procoder/` is adopted; one with an
      `AGENTS.md` naming procoder is adopted; one with neither is not; and
      one with an `AGENTS.md` that never mentions procoder is not.
- [ ] [S-2] In a non-adopting repository, a planted secret, an oversized
      file, a conflict marker, a junk file and an AI-attribution line each
      still block.
- [ ] [S-3] In the same repository, an unformatted file, a missing agent
      rule file, a README without the version, an absent linter, a debt
      marker and a failing suite produce no finding at all.
- [ ] [S-3] In an adopting repository every one of those six still
      reports, unchanged — asserted over the same fixture with `.procoder/`
      added, so the two runs differ only in adoption.
- [ ] [S-4] A repository carrying its own `AGENTS.md` and
      `.github/copilot-instructions.md`, having not adopted procoder, is
      told nothing about either file — no "drift", no "missing".
- [ ] [S-5] The gate's summary line names the mode it ran in, and a
      non-adopting run says how to change it.
- [ ] [S-6] In a non-adopting repository, a secret on a line the commit
      did not touch produces no finding, and the same secret on a line the
      commit added blocks.
- [ ] [S-6] The same holds for a conflict marker: pre-existing, silent;
      introduced by this commit, blocking.
- [ ] [S-6] An oversized file and a junk file that the commit introduces
      still block in a non-adopting repository, because those are about the
      file rather than its contents.
- [ ] [S-6] In an ADOPTING repository a pre-existing secret in a changed
      file still blocks, unchanged — the narrowing applies to somebody
      else's repository, not to your own.
- [ ] [S-5] `[gate] scope = "universal"` in an adopting repository reduces
      it to the universal checks, and `PROCODER_GATE_SCOPE=adopted` in a
      non-adopting one runs everything.

## Open questions

<!-- none -->

## Decisions

- **D-1: two modes, not a per-domain matrix.** A repository either asked
  for procoder's opinions or it did not. Letting a non-adopting repository
  enable individual domains means a configuration surface for repositories
  that by definition carry no configuration.

- **D-2: `.procoder/` OR an `AGENTS.md` naming procoder.** Both are things
  a repository does deliberately. An installed binary, a plugin, an
  environment variable — none of those are the repository's choice, and
  the contributor's machine is identical in both cases.

- **D-3: the universal set is secrets, oversized files, conflict markers,
  junk files, and attribution.** Each is true about the commit rather than
  about the project's taste: no repository wants a credential, a 12MB
  binary, a `<<<<<<<` marker, a `.DS_Store`, or a trailer nobody wrote.
  Everything else — formatting, linting, agent rules, release hygiene,
  planning, debt, the suite — is a house rule.

- **D-5: in somebody else's repository, only the diff is mine.** The
  reported false positive was a constant 2,500 lines from the change, in a
  file the commit merely touched. Whole-file content scanning is defensible
  in a repository that adopted procoder and asked to hear about its own
  code; in one that did not, it means answering for four thousand lines
  somebody else wrote, and it is what turns `--no-verify` into muscle
  memory. Only the content checks narrow: a file this commit ADDS is this
  commit's entirely, so size and junk stay file-level.

- **D-4: the mode is announced, always.** A gate that quietly checks less
  is the failure this project is built to prevent, and it does not stop
  being that because the reduction was deliberate.
