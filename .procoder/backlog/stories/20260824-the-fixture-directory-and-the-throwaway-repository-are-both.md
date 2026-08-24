# Teardown, verified by absence

Status: done 2026-08-24
Created: 2026-08-24
Epic: e2e-campaign
Sprint: 018-the-whole-campaign-re-run-from-scratch-until-nothing-new-then

## Description

The campaign creates two things that should not outlive it: a fixture
directory full of deliberate secrets and vulnerable manifests, and a
public GitHub repository holding a copy of it. Leaving either behind
turns a test artifact into a liability — a public repository of planted
credentials is exactly the thing procoder exists to complain about.

Both are removed, and the removal is verified by checking they are gone
rather than by asserting that the delete command was run. This is the
same discipline as everywhere else in the epic: a command that reports
success and did nothing is the failure mode being hunted.

The build script survives, since it is what makes the fixture
reproducible, and the campaign report survives, since it is the result.

## Acceptance criteria

- [x] The fixture directory and the throwaway repository are both gone at
      close, verified by their absence rather than by assertion.
- [x] The fixture build script and the campaign report remain, and the
      script still rebuilds a working fixture after teardown.

## Evidence

`scripts/e2e-teardown.sh --repo azrtydxb/procoder-e2e-fixture`: **6 pass,
0 fail, 0 skip.**

- The fixture directory is gone, checked with a test for its existence
  rather than by trusting `rm -rf`'s exit code.
- The build script rebuilt a working fixture after the removal — commit
  `00ea6ca8418fd7e9a6b1a93b4bce18c39bd0c0fd`, the same hash it produced on
  the campaign's first day — and that rebuild was then removed too, so
  teardown leaves nothing behind.
- `scripts/build-e2e-fixture.sh` and
  `.procoder/analysis/e2e-campaign-report.md` both survive, as intended:
  the script is what makes a finding reproducible from nothing, and the
  report is the result.
- The repository no longer answers. Deleted by the repository owner —
  this session's token carries `gist, read:org, repo, workflow,
write:packages` and no `delete_repo`, so procoder's side could not
  perform it and said so rather than reporting a removal it had not done.
  The teardown decides that verdict on whether `gh repo view` still
  answers, never on a delete command's exit code, so it reports the truth
  regardless of who did the deleting.
- Confirmed a second, independent way: `gh api
repos/azrtydxb/procoder-e2e-fixture` returns HTTP 404, and `ls` on the
  fixture path returns "No such file or directory".

**A note on what was at stake.** Before teardown the pushed repository was
cloned fresh and scanned: 0 findings. Every planted defect — the
credential, the SAST case, the vulnerable manifest — was created in the
local fixture and never pushed, so no credential was ever public. The
removal is tidiness rather than remediation, and saying which it was
matters: reporting it as a averted leak would have been a more exciting
claim than the evidence supports.
