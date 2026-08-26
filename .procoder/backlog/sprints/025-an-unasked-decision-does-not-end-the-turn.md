# An unasked decision does not end the turn

Status: done
Created: 2026-08-26

## Goal

Make the rule bind instead of depending on the agent remembering it.

3.2.0 shipped the rule that a decision is not the agent's to make: record
it, ask with the host's question tool, do not bury it at the end of a
report. Hours later the agent that wrote the rule ended a triage report
with three decisions in prose and carried on. The rule was in the tool and
not in the agent.

That is every failure this project has already fixed, once more: a rule
that depends on remembering fails on the day somebody is busy. The credit
check, the falsifier rule and the release controller each won the same
argument.

The Stop hook is where it can be caught, because the end of the turn is
when a decision gets buried.

## The design constraint that shaped everything

A false block fires on ordinary work, at the end of EVERY turn. That is
the #172 and #185 failure at maximum frequency, and it would discredit
every other check procoder makes. So the detector errs toward silence: an
explicit ask rather than a question mark, an interrogative phrase only
when it carries a question mark in the same sentence, never twice on one
message, and silence on every uncertain path.

It caught one false positive during development, from a real message in
the session that built it: "I asked whether you want me to keep it, and
you said yes" — narration about a decision already taken, in the same
words as an ask.

## Verified rather than assumed

The first design read the transcript. The host documents the transcript as
written asynchronously and lagging the current turn, and supplies
`last_assistant_message` on Stop for exactly this. Blocking is exit 2 with
stderr, not a JSON decision field. Both were checked against the
documentation before anything was built, and both would have been
plausible and wrong.
