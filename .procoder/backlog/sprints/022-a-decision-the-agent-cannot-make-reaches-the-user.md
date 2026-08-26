# A decision the agent cannot make reaches the user, and survives

Status: done
Created: 2026-08-26

## Goal

Give a decision the same standing a finding already has.

Procoder's principles say questions are not the agent's to answer, and
`procoder ask` collects them. Both were true only for questions that came
from a finding. A decision — commit or hold, merge now or after, which of
two approaches — had no path anywhere: nothing collected it, nothing
recorded it, and nothing noticed it went unasked.

This sprint was opened by a failure in this repository rather than by a
plan. Two decisions were put to the user as prose at the end of a long
status report, with the work continuing underneath them. The user's
response was that they had not been asked properly, and the investigation
found no rule covering the case and no mention of the host's structured
question tool in any procoder rule file.

Two halves, because the failure has two. The principle changes what the
agent does; the queue makes an unasked decision visible and an answered one
durable. Neither is sufficient: a rule with no record is forgotten at the
next compaction, and a record with no rule is a file nobody writes to.

## Deviation from the chain

Recorded because #186 is about exactly this. The implementation preceded
the backlog seed here: the spec was written first and is honest, but the
epic and stories were seeded after the code was working, so the stories
document the work rather than having directed it. The evidence in them is
real — every test named was run and every mutation applied — but the
ordering was wrong and saying so is cheaper than pretending otherwise.
