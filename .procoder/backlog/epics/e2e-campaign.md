# e2e-campaign

Status: done 2026-08-24
Created: 2026-08-24
Spec: e2e-campaign @ 2ede23c986cc

## Description

Everything procoder knows about whether it works, it learned from one
repository: this one. Go, with procoder installed by its author, a year
of `.procoder/` state, every tool present. Most of procoder's branches
turn on those conditions, so the path a new adopter takes is the path
least exercised.

This epic runs procoder against a repository that is not this one — a
fixture built from `git init`, carrying real source in all twelve
claimed languages, its own tests, CI, docs and manifests. Twice: once
healthy, where any finding is a false alarm, and once seeded with one
deliberate defect per class procoder claims to catch, where anything not
caught is a silent green.

The second pass is the point. Every defect found in the session that
prompted this epic reported success: a spec at fourteen of fourteen with
two features never built, a trademark audit blind to the shape it
scanned for, a `#` inside a quoted value read as a comment. A campaign
that only runs healthy code finds none of that class.

The GitHub-dependent half — `ci --runs`, `copilot-leak`, Pages health,
a real tagged release — runs against a throwaway public repository, and
both it and the fixture are destroyed when the epic closes.
