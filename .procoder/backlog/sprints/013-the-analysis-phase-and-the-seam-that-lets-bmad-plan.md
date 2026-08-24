# The analysis phase, and the seam that lets BMad plan

Status: closed 2026-08-24
Created: 2026-08-24

## Goal

The rest of the epic, in one sprint: the nine stories sprint 012 left.

Two of them finish procoder's own chain at the front. `spec check` has
always judged whether a document is complete, never whether the idea in
it is good — it will pass a thoroughly filled-in specification for the
wrong feature. The analysis phase is where an idea becomes something
worth checking, and `analyze check` holds it to the same standard a spec
is held to, because a phase whose documents nobody has to fill in is a
formality that costs time and buys nothing.

The other seven build the seam. A repository sets `[planning] method =
"bmad"` and procoder stops demanding `.procoder/specs|plans|backlog`,
reading BMad's artifacts instead. The setting moves planning and nothing
else: the gate, the suite, the release controller, the debt ledger and
the rest run identically either way, and one story asserts that with
byte-identical output across the setting rather than trusting it.

What makes this tractable is that procoder's governance never needed to
know where a plan came from. It asks whether there is one, whether it is
complete, and whether a story is done — and both worlds can answer.

## Stories

<!-- Pulled with `procoder sprint pull <story-id>`. -->

## Retro

**What slowed us down.** Nothing did, and that is the observation worth
recording. Nine stories across five packages landed without a single
false start, and the reason is that the spec had already made every hard
call — three answers for a status file rather than two, report the
unknown status by name rather than map it, read the output folder rather
than assume it. Implementation was transcription. The two sprints before
this one each lost time to a decision that had not been made yet;
this one lost none, and the difference was in the spec, not in the code.

**What we change.** Nothing new. Sprint 012's adaptation — a
source-scanning audit is finished when an offender of every shape it
claims to catch has been caught, plus a probe proving it leaves the
legitimate case alone — held and needed no revision. Ten mutations were
run this sprint and every one failed as named, including the one that
matters most: gating a governance leg on the planning method breaks the
byte-identical comparison while every other test stays green.

**Worth keeping.** Writing the seam test to exclude the planning
domain's own findings. The obvious version compares all output across
both settings and is useless — it fails the moment the setting does
anything, which is always, so it would have been deleted or weakened
within a week. Excluding the domain under test and asserting everything
else is identical is what makes it a claim about governance rather than
a claim that two commands are the same command. The general form: a test
that something did not change has to say what was allowed to.

## Result

committed: 9
done: 9 (20260824-a-sprint-status-yaml-that-will-not-parse-produces-a, 20260824-a-status-in-sprint-status-yaml-that-procoder-does-not, 20260824-planning-method-bmad-with-a-fixture-bmad-install-reports, 20260824-planning-method-bmad-with-no-bmad-installed-produces-a, 20260824-planning-method-nonsense-is-a-config-problem-naming-the, 20260824-procoder-analyze-check-refuses-a-hollow-analysis-document-a, 20260824-procoder-doctor-under-method-bmad-names-bmad-s-installed, 20260824-procoder-spec-check-names-the-analysis-document-a-spec-came, 20260824-under-method-bmad-procoder-check-produces-byte-identical)
carried: 0
