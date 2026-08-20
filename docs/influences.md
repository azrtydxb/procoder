# Influences

**An explanation.** Procoder's design draws on ideas proven by two
earlier plugins:
[superpowers](https://github.com/anthropics/claude-plugins-official) and
[ponytail](https://github.com/DietrichGebert/ponytail). This page is the
provenance map — each adopted concept and exactly where it lives in
Procoder — both as credit and as a migration reference for users of the
originals, which Procoder fully replaces.

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

| Concept                                                                                                                                                                                   | Where it lives in Procoder                                                                                                                                                         |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The build ladder — does it need to exist → reuse → stdlib → platform → dependency → one line → minimum code — and "the ladder runs after you understand the problem, never instead of it" | `procoder principles`, the default `AGENTS.md` contract, and `.procoder/PRINCIPLES.md` (repo-overridable, [configuration](configuration.md))                                       |
| Deliberate corner-cuts carry a marker naming the ceiling and the revisit trigger; a command harvests the ledger and flags trigger-less rot                                                | `procoder debt` and the `debt:` convention (`[debt]` in config.toml) — the [maintainability domain](domains.md)'s sibling discipline                                               |
| The five-tag over-engineering review (delete / stdlib / native / yagni / shrink), a mandatory replacement per finding, and the honest null result "Lean already. Ship."                   | `/procoder:simplify`                                                                                                                                                               |
| One canonical instruction file, thin per-host adapters, parity and drift checks — the architecture that serves twenty agents from one source                                              | the whole [universal agent layer](portability.md), with the drift guards moved into the binary and riding the gate                                                                 |
| Version-sync must also check the release tag (their v4.8.0 shipped every manifest stale together); content canaries pin load-bearing phrases                                              | the docs domain's version and manifest pinning ([domains](domains.md), documentation section)                                                                                      |
| Never print a number you cannot derive; every claim carries its method in the same visual block; publish retractions                                                                      | the README's "Honesty, by design" section, the perf domain's refusal to ship checks without measurement infrastructure, and this site's habit of stating tested-status per feature |

## Deliberately not adopted

Superpowers' subagent-orchestration machinery and branch-finishing flow
(Procoder's own workflow supersedes them) and its visual-companion
server; ponytail's benchmark scoreboard and its lite/full/ultra level
system (a repo-editable principles file replaces modes).

Escaped-finding lessons are a different ledger: those live in
`.procoder/github/LESSONS.md` and are enforced by `procoder lessons` —
see [the quality chain](quality-chain.md).
