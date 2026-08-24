# the whole campaign re-run from scratch, until nothing new, then teardown

Status: closed 2026-08-24
Created: 2026-08-24

## Goal

Run everything again, from a fixture built from nothing, and find out what
the last four sprints broke.

Eighteen fixes have landed since the first pass, and the single most
likely outcome of a campaign this wide is a fix that closes one command
and opens another. That is why this sprint re-runs whole rather than
around each change: only a full round sees it.

Every finding already carries a regression test, and each of those tests
has had its mutation run and watched fail — but a test proves the defect
does not return, not that the fix left everything else alone. The passes
are what answer that.

The loop ends when a full round of both passes, the hooks and the docs
produces no finding that was not already recorded and fixed. Not when the
list looks short: each round states what it did NOT cover beside what
passed, so a shrinking finding count cannot be mistaken for shrinking
risk.

Then both fixtures go — the directory full of deliberate secrets and the
public repository holding a copy of it — and their absence is checked
rather than asserted. A public repository of planted credentials left
behind is precisely the thing procoder exists to complain about.

## Result

committed: 2
done: 2 (20260824-every-finding-has-a-fix-and-a-regression-test-that-fails, 20260824-the-fixture-directory-and-the-throwaway-repository-are-both)
carried: 0

## Retro

**The round that found nothing was the one worth running.** Eighteen fixes
had landed on top of a fixture none of them was written against, and the
likeliest outcome of a campaign this wide is a fix that closes one command
and opens another. Round two matched round one in every phase. That
silence is a result, not an absence of one — but only because round one's
numbers were written down precisely enough to compare against.

**Re-running whole beat re-running around the change.** Nothing here would
have been caught by re-testing near each fix: the question was whether the
fixes interacted, and only the full round can answer it. It cost about
twenty minutes of wall clock and the reasoning it replaced is the kind
that sounds convincing and is not checkable.

**A scare that was the environment again, and the gap it exposed was
real.** `.procoder/bench/baseline.txt` showed modified with no `--save`
run, which looked exactly like a P-CONTROL violation in the one command
that legitimately writes. It was a stale artifact from an earlier save —
measured, not assumed: neither `go test ./...` nor `procoder bench` touches
the file. But the reason it was frightening is that `bench` and `release`,
the only two commands that CAN write, were absent from the P-CONTROL loop.
Both are in it now, and both write nothing. **A false alarm is worth
following to where it points even after it turns out to be false.**

**Teardown had to be a check, not a command.** `rm -rf` exits 0 whether or
not anything was there, and `gh repo delete` exits non-zero on a
repository already gone. Deciding the verdict on whether the thing still
answers meant the teardown reported the truth when the repository was
removed by somebody else entirely, which is exactly what happened — the
token had no `delete_repo` scope, and the report is identical to the one a
successful self-delete would have produced.

**The adaptation worth keeping: say which kind of thing you found.** The
pushed repository was scanned before removal and carried no credential at
all; every planted defect had lived in the local directory. Reporting the
teardown as an averted leak would have been a better story than the
evidence supports, and the evidence is the thing being reported.
