# A spec's claims are checked by something other than whoever wrote them

Status: complete

## Problem

Almost every sprint deviates from its own spec during implementation. The
chain refuses a spec with no `[S-n]` coverage, a plan with no spec, a story
with no epic — all of it structural. Nothing anywhere asks whether what the
spec says is TRUE, or whether a criterion can be OBSERVED. So the first
time any claim meets the code is at the keyboard, mid-sprint, which is the
most expensive moment to find out.

Sprint 021 is the worked example, and its five deviations are exactly two
kinds.

Claims about current behaviour that nobody checked: S-3 listed formatting
among the domains to scope, without anybody looking at where the code
decides it — the format loop sat before the scope decision, so honouring
that line meant restructuring `RunWith` and repairing four fixtures. And a
criterion was written about narrowing junk findings to the diff, a failure
that cannot happen, because those findings carry no line number.

Criteria naming an observable the fixture cannot produce: "a README without
the version produces no finding" needs a built index, and without one the
docs domain prints `public surface NOT computed` and never reaches the
finding. "A failing suite produces no finding" needs a suite the fixture
does not have. Both are green whatever the code does, which is the same
silent green this project exists to prevent, arriving through the backlog.

Three of the five were caught only because mutation testing forced the
mutation to be applied and watched to fail.

## Users

Whoever writes the spec, who currently has no way to find out they are
wrong until the sprint is underway.

Whoever implements it, who pays for the deviation.

## In scope

- [S-1] A spec that cites a symbol or a repository path which does not
  exist is refused, naming the citation and the line it is on.
- [S-2] An acceptance criterion that does not say how it is observed is
  refused. Naming the command, the test, or the artifact is what makes a
  criterion checkable rather than agreeable.
- [S-3] A criterion whose observable has a known prerequisite — the docs
  domain needs a built index, the suite leg needs tests, the dependency
  scan needs a lockfile — is refused unless the criterion names that
  prerequisite, because without it the criterion passes whatever the code
  does.
- [S-4] The same criterion rules apply to stories, since a story's
  criteria are what the closer asks for evidence against.
- [S-5] Every refusal names what to do about it. A checker that refuses
  without a route is a gate people learn to route around.
- [S-6] The checks are mechanical. Nothing here judges whether prose is
  true; it checks that claims are CITED and criteria are OBSERVABLE, which
  a machine can do and an author reliably cannot do about their own work.

## Out of scope

- Judging prose. "Is this sentence true" is not mechanical, and a checker
  that guessed would be a worse liar than the gap it replaced.
- Executing criteria. Running each criterion's fixture at check time is
  the right eventual answer and is a sprint of its own; this change makes
  the criterion state what would be run.
- Retrofitting existing specs. New and edited specs are checked; the
  archive is not rewritten to satisfy a rule written after it.

## Constraints

- **No new refusal without a route out.** Every one names the fix.
- **Mechanical only.** A check whose verdict depends on reading meaning
  does not go in.
- **The existing verdicts do not move.** A spec that passes today and
  cites nothing false, with criteria that name their observables, passes
  unchanged.

## Interfaces

| Surface                              | Behaviour                                                                                |
| ------------------------------------ | ---------------------------------------------------------------------------------------- |
| `procoder spec check`                | Adds citation resolution and criterion observability. Refuses with the line and the fix. |
| `procoder backlog check`             | Applies the criterion rules to stories.                                                  |
| A citation that resolves             | Silent.                                                                                  |
| A criterion naming a command or test | Silent.                                                                                  |

## Data

Nothing stored. Citations are resolved against the working tree; the
prerequisite table is a constant in the checker.

## Edge cases

- **A citation in a fenced code block or an Edge cases example.** Prose
  about a symbol that deliberately does not exist — "a `FooBar` that was
  never written" — would be refused wrongly. Code fences are excluded, and
  the check applies to the sections that make claims, not to every line.
- **A citation to a symbol in another language**, or to a tool
  (`gofmt`, `kubeconform`). Only repository paths and Go-shaped
  `pkg.Symbol` citations are resolved; anything else is left alone.
- **A criterion that is genuinely unobservable by command** — a
  documentation promise, a wording rule. Naming the artifact counts as
  naming the observable.
- **A spec written before this check existed.** It is checked when next
  edited, and its failures name lines rather than demanding a rewrite.

## Failure modes

- **False refusals teach `--no-verify`.** The failure this whole project
  keeps relearning (#172, #185). Held by resolving only citations that are
  unambiguous, and by excluding fenced blocks.
- **The check passes everything**, because the patterns match nothing real.
  Held by running sprint 021's own spec through it and requiring the
  defects it was built for to be reported.
- **The prerequisite table rots** as domains change. It names the domain
  and the prerequisite in one place, and a domain whose prerequisite
  changed reports against a table a person can read.

## Acceptance criteria

- [ ] [S-1] `procoder spec check` over a fixture spec citing
      `nosuchpkg.NoSuchSymbol` and `internal/nowhere/absent.go` refuses,
      exits 2, and names both citations with their line numbers.
- [ ] [S-1] The same command over a spec citing `gitx.Attribution` and
      `internal/gate/gate.go` — both of which exist — is silent about
      citations.
- [ ] [S-2] `procoder spec check` over a fixture whose criterion reads "the
      gate is correct" refuses, naming that criterion and saying a
      criterion must name the command, test or artifact that observes it.
- [ ] [S-2] A criterion naming `procoder check` or a `Test...` function is
      accepted.
- [ ] [S-3] A criterion mentioning the docs domain without naming a built
      index is refused, and the refusal names the index as the missing
      prerequisite; the same criterion with `procoder index build` named is
      accepted.
- [ ] [S-4] `procoder backlog close story <id>` says so when a criterion
      names no observable, asserted by
      `TestAStoryCriterionWithNoObservableIsCalledOut`. It reports rather
      than refuses: the refusal belongs at the draft spec, before the
      sprint opens, and refusing at close would retrofit the rule onto
      every story already in flight.
- [ ] [S-5] Every refusal introduced here carries a fix in its text,
      asserted by `TestEveryRefusalNamesTheFix`, which reads the messages
      rather than trusting inspection.
- [ ] [S-6] Run against `.procoder/specs/adoption-aware-gate.md` — the
      spec whose deviations motivated this — the check reports the
      criteria that name no observable. Which of the five defects are
      mechanically reachable is stated in the evidence rather than
      assumed, so a defect this cannot reach is recorded as such rather
      than quietly dropped.

## Open questions

## Decisions

- **Citation-resolution rather than prose-judging.** A machine can check
  that `gitx.Attribution` exists; it cannot check that "the gate runs
  formatting first" is true. Requiring the claim to cite something turns
  the second into the first for the claims that matter.
- **Refuse rather than warn.** A warning on a spec is read once and never
  again, and the chain's other controllers refuse.
- **The archive is not retrofitted.** A rule written today applied
  retroactively to forty specs would produce a wall of refusals nobody
  reads, which is how a check gets switched off.
