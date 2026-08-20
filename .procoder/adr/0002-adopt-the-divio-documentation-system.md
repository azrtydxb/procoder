# 0002 — adopt the Divio documentation system

Status: accepted
Date: 2026-08-20

## Context

Procoder's documentation domain enforced that documentation EXISTS —
required files, required badges, a README first screen, no broken links,
no stale references. It said nothing about what a page should be. The
result is visible on the site: `docs/workflow.md` and
`docs/quality-chain.md` each teach a newcomer, list steps for a competent
user, and argue for the design, inside one page. A reader arrives with
one of those three needs and pays for the other two.

That is not a writing-quality problem to be fixed page by page. It is a
missing structural rule, and without one every new page re-invents its
own shape and drifts toward the same mixture.

## Decision

Adopt the Divio documentation system
(https://docs.divio.com/documentation-system/) as the structural law for
documentation, and record it in `.procoder/docs/RULES.md` — the file the
docs domain already reads, and which any repository can override
(D-OVERRIDE).

Four kinds, never mixed: **tutorial** (teaches a newcomer, must run
exactly as written), **how-to guide** (one stated goal, steps,
competence assumed), **reference** (describes, never persuades, complete
rather than interesting), **explanation** (why it is like this; no
steps). The kind is decided before the first line and stated in the
page's opening sentence.

Alongside it, a writing-craft section: answer first, examples over prose
about examples, real names rather than `foo`, sentences under fifteen
words, scannable structure, the searchable synonym included, and explicit
"common pitfalls" where a feature has a known misuse.

Alternatives considered:

- **Style guide only** (the craft rules without the four types). Rejected:
  the mixed-purpose page is the actual failure here, and no amount of
  short sentences fixes a page serving two readers.
- **Enforce the four types mechanically** — a front-matter `type:` field
  the gate checks. Rejected for now: the classification is a judgment,
  and a machine check would either be trivially satisfiable or wrong.
  Reconsider if pages drift back to mixed purposes anyway.
- **Leave it to the writer.** Rejected: that is the status quo that
  produced the mixture.

## Consequences

Easier: a writer decides one thing (which kind) and the shape follows.
Review gets a concrete question — "which of the four is this?" — instead
of a taste argument. Readers with a single need stop paying for the other
three.

Harder: the existing site does not comply. `workflow.md` and
`quality-chain.md` are mixed and need splitting; `getting-started.md`
must become a tutorial that actually runs clean end to end, which is a
stronger promise than it makes today. Until that work lands, procoder's
own documentation is an exception to a rule procoder ships — recorded
here so it is a known debt rather than a quiet one.

The system is guidance, not a gate. Nothing about this blocks a commit,
which means it holds only as long as it is applied deliberately.
