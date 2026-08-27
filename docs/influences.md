# Influences

**An explanation.** Procoder absorbed the work of three earlier tools:
[superpowers](https://github.com/anthropics/claude-plugins-official),
[ponytail](https://github.com/DietrichGebert/ponytail), and
[serena](https://github.com/oraios/serena). This page is the provenance
map — each adopted idea and exactly where it lives now — written both as
credit and as a migration reference.

**If you run any of the three, you no longer need to.** Procoder
replaces superpowers and ponytail outright, and replaces serena for the
languages it indexes — [that section](#from-serena) has the detail.
Running them alongside Procoder means two sets of instructions competing
for the same agent, which is worse than either alone.

## From superpowers

| Concept                                                                                                                                                                                  | Where it lives in Procoder                                                                                                                                     |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Plans written for an engineer with zero context: Files, exact Interfaces, test-first steps, no placeholders (from its writing-plans skill)                                               | `procoder plan` + `/procoder:plan` — the [quality chain](quality-chain.md)'s middle link, with a controller that blocks what the original only advised against |
| Classify before designing: spike / bounded / architectural, with a one-way ratchet and an approval gate that never scales down (from brainstorming)                                      | `/procoder:spec`'s opening classification                                                                                                                      |
| No completion claims without fresh verification evidence; the red-green proof for regression tests; an agent's "done" is a claim (from verification-before-completion)                   | `/procoder:todo`'s evidence rules — enforced by `todo close`, which refuses instead of reminding                                                               |
| Root cause before any fix; one hypothesis at a time; three failed fixes means the architecture is wrong (from systematic-debugging)                                                      | `/procoder:debug`                                                                                                                                              |
| Red before green with mandatory evidence; every test names the break it catches; the mutation check; mock discipline (from test-driven-development and its writing-good-tests reference) | `/procoder:tdd`                                                                                                                                                |
| Verify review feedback before implementing; ask when any comment is unclear; facts instead of gratitude (from receiving-code-review)                                                     | `/procoder:merge`'s review-receiving rules                                                                                                                     |
| Re-inject the meta-instructions at every session start so context loss cannot erase them                                                                                                 | the principles SessionStart hook                                                                                                                               |

## From ponytail

| Concept                                                                                                                                                                                       | Where it lives in Procoder                                                                                                                                                         |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The build ladder — does it need to exist → reuse → stdlib → platform → dependency → one line → minimum code — and "the ladder runs after you understand the problem, never instead of it"     | `procoder principles`, the default `AGENTS.md` contract, and `.procoder/PRINCIPLES.md` (repo-overridable, [configuration](configuration.md))                                       |
| Deliberate corner-cuts carry a marker naming the ceiling and the revisit trigger; a command harvests the ledger and flags trigger-less rot                                                    | `procoder debt` and the `debt:` convention (`[debt]` in config.toml) — the [maintainability domain](domains.md)'s sibling discipline                                               |
| The five-tag over-engineering review (delete / stdlib / native / yagni / shrink), a mandatory replacement per finding, and the null result "Lean already. Ship." when there is nothing to cut | `/procoder:simplify`                                                                                                                                                               |
| One canonical instruction file, thin per-host adapters, parity and drift checks — the architecture that serves twenty agents from one source                                                  | the whole [universal agent layer](portability.md), with the drift guards moved into the binary and riding the gate                                                                 |
| Version-sync must also check the release tag (their v4.8.0 shipped every manifest stale together); content canaries pin load-bearing phrases                                                  | the docs domain's version and manifest pinning ([domains](domains.md), documentation section)                                                                                      |
| Never print a number you cannot derive; every claim carries its method in the same visual block; publish retractions                                                                          | the README's "Honesty, by design" section, the perf domain's refusal to ship checks without measurement infrastructure, and this site's habit of stating tested-status per feature |

## From serena

Serena is a different kind of tool from the other two: an MCP server that
gives an agent language-server power — symbol-level navigation and
symbol-level editing — instead of a set of skills. Procoder took the
navigation half into the binary and deliberately refused the editing
half.

| Concept                                                                                  | Where it lives in Procoder                                                                                                                                            |
| ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Symbol-level navigation instead of grep: find a symbol, its references, a file's outline | `procoder index find` / `refs` / `outline` / `callers` — two tiers, universal-ctags broad and SCIP precise ([the code index](domains.md))                             |
| What implements this interface                                                           | `procoder index impls` (precise tier only, and it says so when the tier is missing)                                                                                   |
| Cross-file rename computed by the language's own engine rather than by pattern           | `procoder index rename <symbol> <new>` — printed as a reviewable unified diff, Go via gopls. **Nothing is written**: you review and apply it, then verify with `refs` |
| Type errors the linter does not catch, because it never compiles the code                | `procoder lint --types` — `tsc --noEmit` for TypeScript, pyright for Python                                                                                           |
| Onboarding: a first pass that learns an unfamiliar codebase                              | `procoder audit` and [how to onboard an existing codebase](how-to-onboard.md)                                                                                         |
| Project memories that survive a lost context                                             | `.procoder/` — specs, plans, ADRs, the lessons ledger. Committed Markdown a human reviews in a PR, not an opaque store only the agent can read                        |
| "Think about what you are doing" meta-tools that re-focus a drifting agent               | the principles injected at every session start, and controllers that refuse rather than remind                                                                        |

**Where it applies.** The precise tier covers the languages with a SCIP
indexer; rename is computed by gopls and therefore answers for Go. A
language without an engine gets the reference worksheet, labelled as
such rather than presented as a rename. For those cases the index skill
points at your host's own language-server tool, and back to the index
for repo-wide sweeps.

**What Procoder refused.** Serena's symbol-level editing tools —
`replace_symbol_body`, `insert_after_symbol` and their siblings — write
to your code. That is precisely the line
[P-CONTROL](architecture.md#contract-1-p-control-the-agent-stays-in-control)
draws: Procoder computes the change and hands it over; the agent
reviews it and writes it. A rename you never saw is a change nobody
reviewed.

## From BMad Method

**A different relationship from the three above.** Those were replaced.
[BMad Method](https://github.com/bmad-code-org/BMAD-METHOD) was not, and
the choice was deliberate: procoder learned the half it had never built,
and separately learned to get out of the way for repositories that would
rather run the real thing. Both halves shipped in 2.0.0.

So this section is credit without a migration note. If you run BMad Method,
keep running it — `[planning] method = "bmad"` exists so that the two do
not compete for the same artifacts.

| Concept                                                                                                   | Where it lives in Procoder                                                                                                                                                                                                                                                               |
| --------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A phase before the spec that asks whether the _idea_ is good, not only whether the _document_ is complete | `procoder analyze` — `brief`, `where`, `list`, `check`, held to the same non-hollow standard `spec check` holds a spec to                                                                                                                                                                |
| A repository's planning artifacts can live outside `.procoder/` entirely, governed by their own tool      | `[planning] method = "bmad"` — the seam in `internal/planning/bmad.go`, which reads BMad's own `sprint-status.yaml` and its `output_folder` setting rather than copying them, and reports an unrecognised status by name instead of quietly mapping it to something procoder understands |
| Governance reaches the same verdict about the same code whichever methodology planned it                  | `TestGovernanceIsUntouchedByThePlanningMethod` in `internal/gitcmd/seam_test.go` — asserted by a test rather than promised in prose                                                                                                                                                      |

**What procoder did not take.** BMad's agent personas — the analyst, the
architect, the scrum master — are the part it models most distinctively,
and procoder has no equivalent. Its unit is a domain with a controller that
refuses, not a role with a voice. Naming that plainly is the point of this
page: the personas are not missing by oversight, and a reader comparing the
two should know where the resemblance stops.

## Deliberately not adopted

Superpowers' subagent-orchestration machinery and branch-finishing flow
(Procoder's own workflow supersedes them) and its visual-companion
server; ponytail's benchmark scoreboard and its lite/full/ultra level
system (a repo-editable principles file replaces modes); serena's
symbol-level write tools and its always-on MCP server (Procoder is one
binary the hooks call, with no process to keep running).

Escaped-finding lessons are a different ledger: those live in
`.procoder/github/LESSONS.md` and are enforced by `procoder lessons` —
see [the quality chain](quality-chain.md).
