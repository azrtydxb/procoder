# the whole campaign re-run from scratch, until nothing new, then teardown

Status: active
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
