# A spec's claims are checked by something other than whoever wrote them

Status: done
Created: 2026-08-26

## Goal

Stop the chain passing documents that are complete and wrong.

Every controller in the chain is structural: sections present, questions
closed, `[S-n]` ids covered. All of it can be satisfied by a spec that
asserts things the code does not do and promises criteria no fixture can
produce — and sprint 021 was exactly that, five times, in a spec that was
long and careful.

Two mechanical checks, because "is this sentence true" is not mechanical
and a checker that guessed would lie more confidently than the gap it
replaced. A claim that CITES something can have the citation resolved. A
criterion can be required to name how it is observed.

## What it caught while being built

Three things, all in its own work, which is the strongest evidence it does
something:

- Its own spec cited `procoder backlog check`, a command that does not
  exist. The command-citation check was added because of it — and then
  did not catch it, because only the top-level command resolves and
  `backlog` is real. Recorded as a stated limitation rather than left to
  be found later.
- Its own first bug: criteria wrap, and reading only the first line
  refused three criteria in its own spec whose observable sat on the
  continuation.
- Its own first false positive: `AGENTS.md` parsed as the `pkg.Symbol`
  shape, so a real file in the repository root was refused.

## Honest reach

Of the five sprint-021 deviations that motivated this, it catches the two
that are criteria-with-no-observable. The other three — a claim about
where the code decides something, a criterion describing an impossible
failure, and a zero-value collision that makes a fixture unable to
distinguish two outcomes — are not mechanically reachable and are recorded
as such in the story evidence rather than quietly counted as covered.
