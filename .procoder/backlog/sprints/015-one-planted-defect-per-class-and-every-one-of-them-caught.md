# one planted defect per class, and every one of them caught

Status: active
Created: 2026-08-24

## Goal

Break the fixture on purpose, one defect at a time, and find out what
procoder does not see.

Sprint 014 established that procoder says nothing wrong about correct
code — forty-three passes, no false alarms. That is worth exactly as much
as this sprint proves it is: a gate that finds nothing on healthy code
and also nothing on a planted defect is not clean, it is silent, and
silence is the shape of every bug this campaign has turned up so far.

So the fixture gets one deliberate defect per class procoder claims to
catch — unformatted source in each of the twelve languages, a lint
finding, a hardcoded secret, a SAST finding, a manifest pinning a known
CVE, a conflict marker, an oversized file, AI attribution in a commit
message, a debt marker with no revisit trigger, agent rules drifted from
the principles, and a doc reference pointing at nothing.

Each must be caught, by the command that owns it, and named specifically
enough that somebody could fix it without already knowing what was
planted. Anything not caught is the finding.

The security half is proved in both directions: each planted defect
blocks at the documented severity, and then stops blocking when the
documented configuration relaxes it — because a knob that quietly does
nothing looks exactly like a knob that works.
