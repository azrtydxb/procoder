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

## Reach: all five

Four rules, and between them every one of the five sprint-021 deviations
is reported against the spec it came from — asserted by line and by class
in `TestAllFiveMotivatingDefectsAreReported`, so a rule that stopped
working cannot be hidden by another still firing.

- Defects 3 and 4, criteria naming an observable no fixture produces:
  caught by the observable rule and the prerequisite table.
- Defect 1, the formatting claim nobody looked up: caught by requiring a
  promise that names a domain to cite where it lives. The rule does not
  verify the claim — it puts the author in the file, which is where the
  discovery happens.
- Defects 2 and 5, criteria that cannot fail: caught by requiring each
  criterion to say what would make it fail. You cannot state that without
  constructing the case that separates pass from fail, and when you
  cannot, that is the answer.

The falsifier rule is the heaviest of the four and the cost is real: every
criterion in every new draft carries a clause it did not before. Drafts
only, so nothing existing is rewritten.
