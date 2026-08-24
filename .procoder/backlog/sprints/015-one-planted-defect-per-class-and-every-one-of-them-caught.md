# one planted defect per class, and every one of them caught

Status: closed 2026-08-24
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

## Result

committed: 2
done: 2 (20260824-a-planted-secret-sast-finding-and-vulnerable-dependency, 20260824-one-deliberate-defect-per-class-procoder-claims-to-catch-is)
carried: 0

## Retro

**Four wrong verdicts this sprint, all from the harness, none from
procoder.** Three were the same shape: a verdict derived from "does this
text appear", where an empty match is indistinguishable from a clean
result. The fourth was different and worth naming on its own — under `set
-o pipefail`, `procoder security | grep -q X` fails whenever procoder
exits 1, which is precisely what procoder does when it finds something.
Two checks that matched perfectly read as checks that failed. Every
assertion greps a file now, so the exit code under test is procoder's and
the match is grep's, and the two can never be confused again.

**Correcting a false pass produced a false skip.** Widening the
absent-tool pattern to any "NOT ..." line naming the file turned Dart into
a NOT RUN, because procoder separately reports "NOT linted — Dart:
procoder has no linter for it yet" about a file whose formatter had caught
the defect perfectly. The fix that mattered was not the pattern but the
order: test the catch first, against the verdict, and only then ask
whether the tool was absent. A file can be unparseable to one domain and
damning to another — the conflict marker is exactly that.

**The adaptation worth keeping: replay the classifier over logs already on
disk.** Both classifier bugs were found in seconds that way, without
re-running a ten-minute pass and without trusting the next version to be
right. The logs are the fixture for the harness.

**A plant can test the wrong thing entirely.** The secret plant used
AWS's documented example access key (the `AKIA…EXAMPLE` one), AWS's own documented example key, which every
scanner allowlists on purpose — so it measured the allowlist, not the
scanner. It also carried a `// nolint` line I had added without thinking,
which measurement showed made no difference. Chasing that misfire is what
found the real defect underneath, so the wrong plant was not wasted, but
it was luck rather than method: a plant has to be verified to be catchable
before its miss means anything.

**A criterion can be wrong rather than unmet.** "Each stops blocking when
the documented configuration relaxes it" could not be satisfied, because
no such relaxation exists — one knob, three values, all of them
strengthenings. Rewriting it in the open, with the measurement that
disproved the premise, is the only version of that move that is not the
failure this project built a scope-coverage gate to prevent.
